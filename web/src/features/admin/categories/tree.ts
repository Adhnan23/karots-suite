import type { CategoryNode } from '@/api/types.gen'

/** Rows to show: with a search, matches plus their ancestors (so a match keeps its
 * context); without one, everything not under a collapsed category. */
export function visibleTree(nodes: CategoryNode[], needle: string, collapsed: ReadonlySet<number>): CategoryNode[] {
  const q = needle.trim().toLowerCase()
  if (!q) return nodes.filter((n) => !n.path.some((id) => collapsed.has(id)))
  const keep = new Set<number>()
  for (const n of nodes) {
    if (n.name.toLowerCase().includes(q)) {
      keep.add(n.id)
      n.path.forEach((id) => keep.add(id))
    }
  }
  return nodes.filter((n) => keep.has(n.id))
}

/** "Food › Fruit › Apple" for a category id ('' if unknown). */
export function pathName(nodes: CategoryNode[], id: number): string {
  const byId = new Map(nodes.map((n) => [n.id, n]))
  const n = byId.get(id)
  if (!n) return ''
  return [...n.path.map((p) => byId.get(p)?.name ?? '?'), n.name].join(' › ')
}

/** Ids of a node and everything under it (a category can't move into these). */
export function subtree(nodes: CategoryNode[], id: number): Set<number> {
  return new Set(nodes.filter((n) => n.id === id || n.path.includes(id)).map((n) => n.id))
}
