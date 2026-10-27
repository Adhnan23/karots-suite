import type { LabelKind } from '@/features/print/labels/kit'
import type { DoseLabel } from '../label'
import { BoxedDose } from './boxed'
import { ClassicDose } from './classic'
import { ClearDose } from './clear'

/** Pharmacy's dose sticker as a label kind (Image mode). */
export const DOSE_KIND: LabelKind<DoseLabel> = {
  kind: 'pharmacy.dose',
  label: 'Dose label',
  sample: { patient: 'Nimal Perera', drug: 'Amoxicillin 500mg', qty: 15, dose: '1-0-1', words: 'Morning · Night', perDay: 2, note: 'After meals' },
  styles: [
    { key: 'classic', label: 'Classic', Template: ClassicDose },
    { key: 'clear', label: 'Clear', Template: ClearDose },
    { key: 'boxed', label: 'Boxed', Template: BoxedDose },
  ],
}
