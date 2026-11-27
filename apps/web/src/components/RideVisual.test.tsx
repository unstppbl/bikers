import { isInaccessible, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { DatePlate } from './RideVisual'

describe('public ride time window', () => {
  it('exposes the departure window while keeping the duplicated day and month decorative', () => {
    render(<DatePlate date="2026-12-01" locale="en" timeWindow="Morning" />)
    expect(isInaccessible(screen.getByText('Morning'))).toBe(false)
    expect(isInaccessible(screen.getByText('01'))).toBe(true)
    expect(isInaccessible(screen.getByText('Dec'))).toBe(true)
  })
})
