// Client-side rules mirror the API's validation so users get instant feedback.
// The server remains the source of truth and re-validates everything.

export const rules = {
  identifier: {
    required: 'Enter your email or username.',
    maxLength: { value: 254, message: 'Email or username is too long.' },
    setValueAs: (v) => v.trim(),
  },
  email: {
    required: 'Email is required.',
    maxLength: { value: 254, message: 'Email is too long.' },
    pattern: { value: /^[^\s@]+@[^\s@]+\.[^\s@]+$/, message: 'Enter a valid email address.' },
    setValueAs: (v) => v.trim().toLowerCase(),
  },
  username: {
    required: 'Username is required.',
    pattern: {
      value: /^[a-z0-9][a-z0-9._-]{2,31}$/,
      message: 'Use 3–32 lowercase letters, numbers, dots, dashes or underscores.',
    },
    setValueAs: (v) => v.trim().toLowerCase(),
  },
  firstName: {
    required: 'First name is required.',
    maxLength: { value: 80, message: 'First name must be at most 80 characters.' },
    validate: (v) => v.trim() !== '' || 'First name is required.',
  },
  lastName: {
    required: 'Last name is required.',
    maxLength: { value: 80, message: 'Last name must be at most 80 characters.' },
    validate: (v) => v.trim() !== '' || 'Last name is required.',
  },
  phone: {
    validate: (v) => !v?.trim() || /^\+?[0-9][0-9 ()-]{5,18}[0-9]$/.test(v.trim()) || 'Enter a valid phone number.',
  },
  password: {
    required: 'Password is required.',
    minLength: { value: 8, message: 'Password must be at least 8 characters.' },
    validate: {
      bytes: (v) => new TextEncoder().encode(v).length <= 72 || 'Password must be at most 72 bytes.',
      mix: (v) => (/\p{L}/u.test(v) && /\d/.test(v)) || 'Password must contain at least one letter and one number.',
    },
  },
}
