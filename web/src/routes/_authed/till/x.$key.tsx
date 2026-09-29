import { createFileRoute, notFound } from '@tanstack/react-router'
import { slots } from '@/plugins/slots'

/** A plugin's till tab (slots.tillTabs). */
export const Route = createFileRoute('/_authed/till/x/$key')({
  loader: ({ params }) => {
    if (!slots.tillTabs.some((t) => t.key === params.key)) throw notFound()
  },
  component: function PluginTab() {
    const { key } = Route.useParams()
    const Page = slots.tillTabs.find((t) => t.key === key)!.Page
    return <Page />
  },
})
