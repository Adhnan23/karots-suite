import { createMemoryHistory, createRootRoute, createRoute, createRouter, Outlet, RouterProvider } from '@tanstack/react-router'
import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { slots } from '@/plugins/slots'
import { SurfacePage, surfaceLoader } from './x.$key.$'

function at(path: string) {
  const root = createRootRoute({ component: Outlet, notFoundComponent: () => <p>Not found</p> })
  const page = createRoute({ getParentRoute: () => root, path: '/x/$key/$', loader: surfaceLoader, component: SurfacePage })
  const router = createRouter({ routeTree: root.addChildren([page]), history: createMemoryHistory({ initialEntries: [path] }) })
  render(<RouterProvider router={router} />)
}

describe('plugin surfaces at /x/<key>', () => {
  afterEach(() => { slots.surfaces.length = 0 })
  it('renders the surface with the rest of the path, without asking who is signed in', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch')
    slots.surfaces.push({ key: 'demo', Surface: ({ path }) => <p>{`path:${path}`}</p> })
    at('/x/demo/a/b')
    expect(await screen.findByText('path:a/b')).toBeTruthy()
    expect(fetchSpy.mock.calls.some(([u]) => String(u).includes('/me'))).toBe(false)
  })
  it('an unknown key is not found', async () => {
    at('/x/nope/a')
    expect(await screen.findByText('Not found')).toBeTruthy()
  })
})
