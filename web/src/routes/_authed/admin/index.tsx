import { createFileRoute } from '@tanstack/react-router'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export const Route = createFileRoute('/_authed/admin/')({
  component: AdminHome,
})

function AdminHome() {
  const { me } = Route.useRouteContext()
  return (
    <Card>
      <CardHeader>
        <CardTitle>Welcome, {me.user.name}</CardTitle>
      </CardHeader>
      <CardContent className="text-sm text-muted-foreground">
        Modules arrive in later milestones.
      </CardContent>
    </Card>
  )
}
