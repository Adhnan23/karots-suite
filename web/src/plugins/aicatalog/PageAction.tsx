import { Link } from '@tanstack/react-router'
import { Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/button'

/** ✨ Suggest groups on the Alternatives page: opens the job here. */
export function SuggestGroups() {
  return (
    <Button asChild variant="secondary" className="h-10 gap-1.5 px-4">
      <Link to="/admin/x/$key/$" params={{ key: 'aicatalog', _splat: 'jobs/groups' }}><Sparkles size={16} /> Suggest groups</Link>
    </Button>
  )
}
