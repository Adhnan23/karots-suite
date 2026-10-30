import { createFileRoute, notFound } from '@tanstack/react-router'
import { slots } from '@/plugins/slots'

export function surfaceLoader({ params }: { params: { key: string; _splat?: string } }) {
  if (!slots.surfaces.some((s) => s.key === params.key)) throw notFound()
}

/** A plugin's full-screen page (slots.surfaces): outside the signed-in shells, no
 * dock, no sign-in check — it handles its own. Its sub-pages ride in the splat. */
export function SurfacePage() {
  const { key, _splat } = Route.useParams()
  const Surface = slots.surfaces.find((s) => s.key === key)!.Surface
  return <Surface path={_splat ?? ''} />
}

export const Route = createFileRoute('/x/$key/$')({ loader: surfaceLoader, component: SurfacePage })
