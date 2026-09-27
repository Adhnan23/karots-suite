import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_authed/till/')({
  component: () => <p className="p-4 text-muted-foreground">Till — M1</p>,
})
