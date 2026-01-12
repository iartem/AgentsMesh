import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { PromoCodeInput } from './PromoCodeInput'

// Mock translation function
const mockT = (key: string) => {
  const translations: Record<string, string> = {
    placeholder: 'Enter promo code',
    validate: 'Validate',
    validating: 'Validating...',
    redeem: 'Redeem',
    redeeming: 'Redeeming...',
    enterCode: 'Please enter a promo code',
    invalid: 'Invalid promo code',
    validateError: 'Failed to validate promo code',
    redeemError: 'Failed to redeem promo code',
    valid: 'Valid promo code',
    plan: 'Plan',
    duration: 'Duration',
    months: 'months',
    confirmRedeem: 'Click redeem to activate',
    // Error codes translations
    'errors.promo_code_not_found': 'Promo code not found',
    'errors.promo_code_not_owner': 'Only owner can redeem',
  }
  return translations[key] || key
}

// Mock the promoCodeApi
vi.mock('@/lib/api/promocode', () => ({
  promoCodeApi: {
    validate: vi.fn(),
    redeem: vi.fn(),
  },
}))

import { promoCodeApi } from '@/lib/api/promocode'

describe('PromoCodeInput', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders input and validate button', () => {
    render(<PromoCodeInput t={mockT} />)

    expect(screen.getByPlaceholderText('Enter promo code')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Validate' })).toBeInTheDocument()
  })

  it('converts input to uppercase', async () => {
    render(<PromoCodeInput t={mockT} />)

    const input = screen.getByPlaceholderText('Enter promo code')
    await userEvent.type(input, 'test123')

    expect(input).toHaveValue('TEST123')
  })

  it('disables validate button when code is empty', () => {
    render(<PromoCodeInput t={mockT} />)

    const button = screen.getByRole('button', { name: 'Validate' })
    expect(button).toBeDisabled()
  })

  it('validates promo code successfully', async () => {
    const mockValidate = vi.mocked(promoCodeApi.validate)
    mockValidate.mockResolvedValue({
      valid: true,
      code: 'TEST123',
      plan_name: 'pro',
      plan_display_name: 'Pro',
      duration_months: 3,
    })

    const onValidate = vi.fn()
    render(<PromoCodeInput onValidate={onValidate} t={mockT} />)

    const input = screen.getByPlaceholderText('Enter promo code')
    await userEvent.type(input, 'TEST123')

    const button = screen.getByRole('button', { name: 'Validate' })
    await userEvent.click(button)

    await waitFor(() => {
      expect(screen.getByText('Valid promo code')).toBeInTheDocument()
    })

    expect(screen.getByText(/Plan: Pro/)).toBeInTheDocument()
    expect(screen.getByText(/Duration: 3 months/)).toBeInTheDocument()
    expect(onValidate).toHaveBeenCalledWith({
      valid: true,
      code: 'TEST123',
      plan_name: 'pro',
      plan_display_name: 'Pro',
      duration_months: 3,
    })
  })

  it('shows error for invalid promo code', async () => {
    const mockValidate = vi.mocked(promoCodeApi.validate)
    mockValidate.mockResolvedValue({
      valid: false,
      code: 'INVALID',
      message_code: 'promo_code_not_found',
    })

    render(<PromoCodeInput t={mockT} />)

    const input = screen.getByPlaceholderText('Enter promo code')
    await userEvent.type(input, 'INVALID')

    const button = screen.getByRole('button', { name: 'Validate' })
    await userEvent.click(button)

    await waitFor(() => {
      expect(screen.getByText('Promo code not found')).toBeInTheDocument()
    })
  })

  it('shows error on validate API failure', async () => {
    const mockValidate = vi.mocked(promoCodeApi.validate)
    mockValidate.mockRejectedValue(new Error('Network error'))

    render(<PromoCodeInput t={mockT} />)

    const input = screen.getByPlaceholderText('Enter promo code')
    await userEvent.type(input, 'TEST123')

    const button = screen.getByRole('button', { name: 'Validate' })
    await userEvent.click(button)

    await waitFor(() => {
      expect(screen.getByText('Failed to validate promo code')).toBeInTheDocument()
    })
  })

  it('redeems promo code after validation', async () => {
    const mockValidate = vi.mocked(promoCodeApi.validate)
    const mockRedeem = vi.mocked(promoCodeApi.redeem)

    mockValidate.mockResolvedValue({
      valid: true,
      code: 'TEST123',
      plan_name: 'pro',
      plan_display_name: 'Pro',
      duration_months: 3,
    })

    mockRedeem.mockResolvedValue({
      success: true,
      plan_name: 'pro',
      duration_months: 3,
      new_period_end: '2025-04-01T00:00:00Z',
      message_code: 'promo_code_redeem_success',
    })

    const onRedeemSuccess = vi.fn()
    render(<PromoCodeInput onRedeemSuccess={onRedeemSuccess} t={mockT} />)

    // First validate
    const input = screen.getByPlaceholderText('Enter promo code')
    await userEvent.type(input, 'TEST123')

    const validateButton = screen.getByRole('button', { name: 'Validate' })
    await userEvent.click(validateButton)

    await waitFor(() => {
      expect(screen.getByText('Valid promo code')).toBeInTheDocument()
    })

    // Then redeem
    const redeemButton = screen.getByRole('button', { name: 'Redeem' })
    await userEvent.click(redeemButton)

    await waitFor(() => {
      expect(onRedeemSuccess).toHaveBeenCalledWith({
        success: true,
        plan_name: 'pro',
        duration_months: 3,
        new_period_end: '2025-04-01T00:00:00Z',
        message_code: 'promo_code_redeem_success',
      })
    })

    // Input should be cleared after successful redeem
    expect(input).toHaveValue('')
  })

  it('shows error on redeem failure', async () => {
    const mockValidate = vi.mocked(promoCodeApi.validate)
    const mockRedeem = vi.mocked(promoCodeApi.redeem)

    mockValidate.mockResolvedValue({
      valid: true,
      code: 'TEST123',
      plan_name: 'pro',
      plan_display_name: 'Pro',
      duration_months: 3,
    })

    mockRedeem.mockResolvedValue({
      success: false,
      message_code: 'promo_code_not_owner',
    })

    render(<PromoCodeInput t={mockT} />)

    const input = screen.getByPlaceholderText('Enter promo code')
    await userEvent.type(input, 'TEST123')

    const validateButton = screen.getByRole('button', { name: 'Validate' })
    await userEvent.click(validateButton)

    await waitFor(() => {
      expect(screen.getByText('Valid promo code')).toBeInTheDocument()
    })

    const redeemButton = screen.getByRole('button', { name: 'Redeem' })
    await userEvent.click(redeemButton)

    await waitFor(() => {
      expect(screen.getByText('Only owner can redeem')).toBeInTheDocument()
    })
  })

  it('disables input and button when disabled prop is true', () => {
    render(<PromoCodeInput disabled={true} t={mockT} />)

    const input = screen.getByPlaceholderText('Enter promo code')
    const button = screen.getByRole('button', { name: 'Validate' })

    expect(input).toBeDisabled()
    expect(button).toBeDisabled()
  })

  it('handles Enter key to validate', async () => {
    const mockValidate = vi.mocked(promoCodeApi.validate)
    mockValidate.mockResolvedValue({
      valid: true,
      code: 'TEST123',
      plan_name: 'pro',
      plan_display_name: 'Pro',
      duration_months: 3,
    })

    render(<PromoCodeInput t={mockT} />)

    const input = screen.getByPlaceholderText('Enter promo code')
    await userEvent.type(input, 'TEST123')
    await userEvent.keyboard('{Enter}')

    await waitFor(() => {
      expect(mockValidate).toHaveBeenCalledWith('TEST123')
    })
  })

  it('clears validation when code changes', async () => {
    const mockValidate = vi.mocked(promoCodeApi.validate)
    mockValidate.mockResolvedValue({
      valid: true,
      code: 'TEST123',
      plan_name: 'pro',
      plan_display_name: 'Pro',
      duration_months: 3,
    })

    render(<PromoCodeInput t={mockT} />)

    const input = screen.getByPlaceholderText('Enter promo code')
    await userEvent.type(input, 'TEST123')

    const button = screen.getByRole('button', { name: 'Validate' })
    await userEvent.click(button)

    await waitFor(() => {
      expect(screen.getByText('Valid promo code')).toBeInTheDocument()
    })

    // Change the code
    await userEvent.clear(input)
    await userEvent.type(input, 'NEW')

    // Validation message should be cleared
    expect(screen.queryByText('Valid promo code')).not.toBeInTheDocument()
  })
})
