// Marks and results helpers. Every grade, GPA and total shown comes from the
// backend (service/result_service.go); these helpers only format and give
// early feedback while typing.

export const SHEET_STATUS_META = {
  not_started: { label: 'Not started', badge: 'bg-slate-50 text-slate-500 ring-slate-400/15', dot: 'bg-slate-300' },
  draft: { label: 'Draft', badge: 'bg-slate-100 text-slate-700 ring-slate-500/15', dot: 'bg-slate-400' },
  submitted: { label: 'Submitted', badge: 'bg-amber-50 text-amber-800 ring-amber-600/20', dot: 'bg-amber-500' },
  verified: { label: 'Verified', badge: 'bg-brand-50 text-brand-700 ring-brand-600/15', dot: 'bg-brand-500' },
  finalized: { label: 'Finalized', badge: 'bg-emerald-50 text-emerald-700 ring-emerald-600/15', dot: 'bg-emerald-500' },
}

export const STAGES = [
  { key: 'pending', label: 'Marks' },
  { key: 'subjects_processed', label: 'Subject results' },
  { key: 'processed', label: 'Overall results' },
  { key: 'verified', label: 'Verified' },
  { key: 'published', label: 'Published' },
]

export const STAGE_LABEL = {
  pending: 'Waiting for marks',
  subjects_processed: 'Subject results processed',
  processed: 'Overall results processed',
  verified: 'Verified, ready to publish',
  published: 'Published',
  correction: 'Correction in progress',
}

export const GRADE_SCALE = [
  ['80–100%', 'A+', '5.00'],
  ['70–79%', 'A', '4.00'],
  ['60–69%', 'A-', '3.50'],
  ['50–59%', 'B', '3.00'],
  ['40–49%', 'C', '2.00'],
  ['33–39%', 'D', '1.00'],
  ['0–32%', 'F', '0.00'],
]

/** 79.5 -> "79.5", 80 -> "80". */
export const formatMarks = (v) => (v == null ? '—' : String(Number(v)))

/** 4.666 -> "4.67" (GPA and grade points always show two decimals). */
export const formatGpa = (v) => (v == null ? '—' : Number(v).toFixed(2))

/** 85 -> "85.00%". */
export const formatPct = (v) => (v == null ? '—' : `${Number(v).toFixed(2)}%`)

/** Parses typed marks: "", or a non-negative number with at most two decimals. */
export function parseMarksInput(raw) {
  const s = String(raw ?? '').trim()
  if (s === '') return { empty: true }
  if (!/^\d{1,6}(\.\d{1,2})?$/.test(s)) return { error: s.startsWith('-') ? "Can't be negative" : 'Up to two decimals' }
  return { value: Math.round(Number(s) * 100) }
}

/** Hundredths -> display string. */
export const fromHundredths = (h) => String(h / 100)

export const gradeTone = (grade) =>
  grade === 'F' ? 'text-rose-700' : grade === 'A+' || grade === 'A' ? 'text-emerald-700' : 'text-slate-800'
