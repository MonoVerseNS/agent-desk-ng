import type { Metadata } from "next"

import { AuthProvider } from "@/components/auth-provider"
import { ApiErrorProvider } from "@/components/api-error-provider"
import { ConfirmProvider } from "@/components/confirm-provider"
import { ImageLightboxProvider } from "@/components/image-lightbox"
import { ThemeProvider } from "@/components/theme-provider"
import { TooltipProvider } from "@/components/ui/tooltip"
import { Toaster } from "@/components/ui/sonner"
import { appFontVariables } from "@/lib/fonts"
import { AppI18nProvider } from "@/i18n/provider"

import "./dashboard.css"
import "md-editor-rt/lib/style.css"
import "@/styles/main.scss"

const paletteScript = `
try {
  var palette = window.localStorage.getItem("dashboard_palette");
  document.documentElement.dataset.palette = palette === "plain" || palette === "blue" || palette === "green" || palette === "gray" ? palette : "plain";
} catch (_) {
  document.documentElement.dataset.palette = "plain";
}
`

export const metadata: Metadata = {
  title: "AI Customer Service Admin",
  description: "AI Customer Service Admin",
}

export default function DashboardRootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html lang="en-US" className={appFontVariables} suppressHydrationWarning>
      <body
        className="antialiased font-sans"
      >
        <script dangerouslySetInnerHTML={{ __html: paletteScript }} />
        <AppI18nProvider>
          <ThemeProvider>
            <AuthProvider>
              <ConfirmProvider>
                <ImageLightboxProvider>
                  <TooltipProvider>
                    <ApiErrorProvider>{children}</ApiErrorProvider>
                    <Toaster position="top-center" richColors />
                  </TooltipProvider>
                </ImageLightboxProvider>
              </ConfirmProvider>
            </AuthProvider>
          </ThemeProvider>
        </AppI18nProvider>
      </body>
    </html>
  )
}
