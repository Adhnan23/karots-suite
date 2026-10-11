import { createFileRoute, notFound } from '@tanstack/react-router'
import { slots } from '@/plugins/slots'

/** A plugin's admin page (slots.adminPages); its own sub-pages ride in the splat. */
export const Route = createFileRoute('/_authed/admin/x/$key/$')({
  loader: ({ params }) => {
    if (!slots.adminPages.some((p) => p.key === params.key)) throw notFound()
  },
  component: function PluginAdminPage() {
    const { key, _splat } = Route.useParams()
    const Page = slots.adminPages.find((p) => p.key === key)!.Page
    return <Page path={_splat ?? ''} />
  },
})
