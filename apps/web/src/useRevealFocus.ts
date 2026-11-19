import { useEffect, useRef } from 'react'

type Box = Pick<DOMRectReadOnly, 'top' | 'right' | 'bottom' | 'left'>

// The centre of the area where two boxes overlap, or null when they do not. A hidden bar has an empty box.
function overlapCentre(a: Box, b: Box) {
  const top = Math.max(a.top, b.top)
  const bottom = Math.min(a.bottom, b.bottom)
  const left = Math.max(a.left, b.left)
  const right = Math.min(a.right, b.right)
  return top < bottom && left < right ? { x: (left + right) / 2, y: (top + bottom) / 2 } : null
}

// Whether a bar covers part of the focused element: the bar overlaps it and is the topmost thing there, so a dialog
// or menu above the bar does not count. Only focus the browser shows counts: a tapped button stays under the finger,
// so the tap still ends as a click on it, while keyboard focus and text fields are revealed.
export function hiddenByBar(target: Element, bars: readonly (Element | null)[], topmostAt: (x: number, y: number) => Element | null) {
  if (!target.matches(':focus-visible')) return false
  const box = target.getBoundingClientRect()
  return bars.some((bar) => {
    if (!bar || bar.contains(target)) return false
    const point = overlapCentre(box, bar.getBoundingClientRect())
    return !!point && bar.contains(topmostAt(point.x, point.y))
  })
}

// Chrome applies the root scroll-padding when it scrolls a focused button into view, but not a focused text field, which
// can stay under the sticky top bar or the phone tab bar. Scrolling the element to the nearest edge applies the padding.
// The default scroll behaviour follows CSS scroll-behavior, which never animates under prefers-reduced-motion.
export function useRevealFocus() {
  const topbarRef = useRef<HTMLElement>(null)
  const tabBarRef = useRef<HTMLElement>(null)
  useEffect(() => {
    const reveal = (event: FocusEvent) => {
      const target = event.target
      if (target instanceof Element && hiddenByBar(target, [topbarRef.current, tabBarRef.current], (x, y) => document.elementFromPoint(x, y))) target.scrollIntoView({ block: 'nearest' })
    }
    document.addEventListener('focusin', reveal)
    return () => document.removeEventListener('focusin', reveal)
  }, [])
  return { topbarRef, tabBarRef }
}
