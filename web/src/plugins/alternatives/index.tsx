import { Shuffle } from 'lucide-react'
import type { slots as Slots } from '../slots'
import { AlternativesPage } from './AlternativesPage'
import { ProductSection } from './ProductSection'
import { SwapChip } from './SwapChip'

/** Alternatives' React half: the Apps tile and its pages, the product form
 * section and the till's swap chip on a short line (and in the found-at-till prompt). */
export default function alternatives(s: typeof Slots) {
  s.apps.push({ key: 'alternatives', label: 'Alternatives', description: 'Products that do the same job', icon: Shuffle, color: 'var(--brand)', to: '/admin/x/alternatives' })
  s.adminPages.push({ key: 'alternatives', Page: AlternativesPage })
  s.productFormSections.push({ key: 'alternatives', Section: ProductSection })
  s.tillLineHints.push({ key: 'alternatives', Hint: SwapChip })
  s.tillShortHints.push({ key: 'alternatives', Hint: SwapChip })
}
