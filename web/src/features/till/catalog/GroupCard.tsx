import { ChevronRight } from 'lucide-react'
import { motion } from 'motion/react'
import type { TillGroup } from '@/api/types.gen'
import { cn } from '@/lib/utils'
import { TILE } from '../ui'

export function GroupCard({ group, onOpen }: { group: TillGroup; onOpen: () => void }) {
  return (
    <motion.button type="button" onClick={onOpen} whileHover={{ y: -2 }} whileTap={{ scale: 0.97 }} transition={{ duration: 0.12 }}
      className={cn(TILE, 'bg-muted')}>
      <span className="text-2xl leading-none">{group.emoji || '🗂️'}</span>
      <span className="flex items-center justify-between gap-2 text-[14px] font-bold">
        {group.name}
        <span className="flex items-center text-[11.5px] font-semibold text-faint">{group.itemCount} <ChevronRight size={14} /></span>
      </span>
    </motion.button>
  )
}
