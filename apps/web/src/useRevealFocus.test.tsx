import { render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { hiddenByBar, useRevealFocus } from './useRevealFocus'

// A 768 px wide viewport, 900 px high: the top bar ends at 60 and the tab bar starts at 836.
function box(top: number, bottom: number, left = 0, right = 768) {
  return { top, bottom, left, right, x: left, y: top, width: right - left, height: bottom - top, toJSON: () => ({}) } as DOMRect
}

function place(element: Element, rect: DOMRect) {
  vi.spyOn(element, 'getBoundingClientRect').mockReturnValue(rect)
}

function Page({ topbarRef, tabBarRef }: Partial<ReturnType<typeof useRevealFocus>>) {
  return <>
    <header ref={topbarRef} data-testid="topbar"><select aria-label="Language" /></header>
    <main><textarea aria-label="Announcement" /></main>
    <nav ref={tabBarRef} data-testid="tab-bar"><a href="/">Rides</a></nav>
    <div role="dialog"><button>Cancel</button></div>
  </>
}

function Shell() {
  const { topbarRef, tabBarRef } = useRevealFocus()
  return <Page topbarRef={topbarRef} tabBarRef={tabBarRef} />
}

function layout() {
  const topbar = screen.getByTestId('topbar')
  const tabBar = screen.getByTestId('tab-bar')
  place(topbar, box(0, 60))
  place(tabBar, box(836, 900))
  return { topbar, tabBar, field: screen.getByRole('textbox', { name: 'Announcement' }), tab: screen.getByRole('link', { name: 'Rides' }), dialogButton: screen.getByRole('button', { name: 'Cancel' }) }
}

describe('hiddenByBar', () => {
  beforeEach(() => { render(<Page />) })

  it('finds a focused field that runs under the tab bar', () => {
    const { topbar, tabBar, field, tab } = layout()
    place(field, box(796, 896, 24, 744))
    field.focus()
    const topmostAt = vi.fn(() => tab)
    expect(hiddenByBar(field, [topbar, tabBar], topmostAt)).toBe(true)
    expect(topmostAt).toHaveBeenCalledWith(384, 866)
  })

  it('finds a focused field under the top bar', () => {
    const { topbar, tabBar, field } = layout()
    place(field, box(31, 131, 24, 744))
    field.focus()
    expect(hiddenByBar(field, [topbar, tabBar], () => topbar)).toBe(true)
  })

  it('leaves a field that is clear of both bars', () => {
    const { topbar, tabBar, field } = layout()
    place(field, box(400, 500, 24, 744))
    field.focus()
    const topmostAt = vi.fn(() => null)
    expect(hiddenByBar(field, [topbar, tabBar], topmostAt)).toBe(false)
    expect(topmostAt).not.toHaveBeenCalled()
  })

  it('ignores a hidden tab bar', () => {
    const { topbar, tabBar, field } = layout()
    place(tabBar, box(0, 0, 0, 0))
    place(field, box(796, 896, 24, 744))
    field.focus()
    expect(hiddenByBar(field, [topbar, tabBar], () => tabBar)).toBe(false)
  })

  it('ignores focus inside a bar', () => {
    const { topbar, tabBar, tab } = layout()
    place(tab, box(836, 900, 0, 150))
    tab.focus()
    expect(hiddenByBar(tab, [topbar, tabBar], () => tab)).toBe(false)
  })

  it('ignores a bar that a dialog or menu covers', () => {
    const { topbar, tabBar, dialogButton } = layout()
    place(dialogButton, box(820, 864, 24, 744))
    dialogButton.focus()
    expect(hiddenByBar(dialogButton, [topbar, tabBar], () => dialogButton)).toBe(false)
  })

  it('ignores an element without visible focus', () => {
    const { topbar, tabBar, field, tab } = layout()
    place(field, box(796, 896, 24, 744))
    expect(hiddenByBar(field, [topbar, tabBar], () => tab)).toBe(false)
  })
})

describe('useRevealFocus', () => {
  const originalIntoView = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'scrollIntoView')
  const originalFromPoint = Object.getOwnPropertyDescriptor(document, 'elementFromPoint')
  const intoView = vi.fn()

  beforeEach(() => {
    intoView.mockReset()
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', { configurable: true, value: intoView })
  })

  afterEach(() => {
    if (originalIntoView) Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', originalIntoView); else delete (HTMLElement.prototype as { scrollIntoView?: unknown }).scrollIntoView
    if (originalFromPoint) Object.defineProperty(document, 'elementFromPoint', originalFromPoint); else delete (document as { elementFromPoint?: unknown }).elementFromPoint
  })

  it('scrolls a field the tab bar covers to the nearest edge, without an animation of its own', () => {
    render(<Shell />)
    const { field, tab } = layout()
    Object.defineProperty(document, 'elementFromPoint', { configurable: true, value: () => tab })
    place(field, box(796, 896, 24, 744))
    field.focus()
    expect(intoView).toHaveBeenCalledTimes(1)
    expect(intoView.mock.contexts[0]).toBe(field)
    expect(intoView).toHaveBeenCalledWith({ block: 'nearest' })
  })

  it('leaves the page still when a field is clear of the bars', () => {
    render(<Shell />)
    const { field, tab } = layout()
    Object.defineProperty(document, 'elementFromPoint', { configurable: true, value: () => tab })
    place(field, box(400, 500, 24, 744))
    field.focus()
    expect(intoView).not.toHaveBeenCalled()
  })
})
