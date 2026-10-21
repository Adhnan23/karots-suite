"use client"

import { useTheme } from "next-themes"
import { Toaster as Sonner, type ToasterProps } from "sonner"
import { CircleCheckIcon, InfoIcon, TriangleAlertIcon, OctagonXIcon, Loader2Icon } from "lucide-react"

const Toaster = ({ ...props }: ToasterProps) => {
  const { theme = "system" } = useTheme()

  return (
    <Sonner
      theme={theme as ToasterProps["theme"]}
      className="toaster group"
      icons={{
        success: (
          <CircleCheckIcon className="size-4" />
        ),
        info: (
          <InfoIcon className="size-4" />
        ),
        warning: (
          <TriangleAlertIcon className="size-4" />
        ),
        error: (
          <OctagonXIcon className="size-4" />
        ),
        loading: (
          <Loader2Icon className="size-4 animate-spin" />
        ),
      }}
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
          "--border-radius": "var(--radius)",
          // richColors tints follow the palette's money colours, not sonner's own.
          "--success-bg": "color-mix(in srgb, var(--success) 12%, var(--popover))",
          "--success-border": "color-mix(in srgb, var(--success) 35%, var(--popover))",
          "--success-text": "var(--success)",
          "--error-bg": "color-mix(in srgb, var(--danger) 12%, var(--popover))",
          "--error-border": "color-mix(in srgb, var(--danger) 35%, var(--popover))",
          "--error-text": "var(--danger)",
          "--warning-bg": "color-mix(in srgb, var(--warning) 12%, var(--popover))",
          "--warning-border": "color-mix(in srgb, var(--warning) 35%, var(--popover))",
          "--warning-text": "var(--warning)",
          "--info-bg": "color-mix(in srgb, var(--brand-2) 12%, var(--popover))",
          "--info-border": "color-mix(in srgb, var(--brand-2) 35%, var(--popover))",
          "--info-text": "var(--foreground)",
        } as React.CSSProperties
      }
      toastOptions={{
        classNames: {
          toast: "cn-toast",
        },
      }}
      {...props}
    />
  )
}

export { Toaster }
