import { Download } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'

/** Export ▾ CSV / XLSX / ODS: `url` is the list or report's API URL with its
 * live filters; the server answers ?format= with every row. */
export function ExportMenu({ url }: { url: string }) {
  const sep = url.includes('?') ? '&' : '?'
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild><Button variant="outline" className="h-10 gap-1.5 px-4 print:hidden"><Download size={16} /> Export</Button></DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {(['csv', 'xlsx', 'ods'] as const).map((f) => (
          <DropdownMenuItem key={f} asChild><a href={`${url}${sep}format=${f}`}>{f.toUpperCase()}</a></DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
