import { ListPlus } from 'lucide-react'
import { Tag } from '@/features/admin/products/ProductsPage'
import type { slots as Slots } from '../slots'
import { FieldsPage } from './FieldsPage'

function ProductPlusPage() {
  return <FieldsPage fillLink={(f) => <Tag tone="warn">{`${f.missing} missing`}</Tag>} />
}

/** Product Plus' React half: the Apps tile and its page. */
export default function productplus(s: typeof Slots) {
  s.apps.push({ key: 'productplus', label: 'Product Plus', description: 'Extra fields for your products', icon: ListPlus, color: 'var(--brand)', to: '/admin/x/productplus' })
  s.adminPages.push({ key: 'productplus', Page: ProductPlusPage })
}
