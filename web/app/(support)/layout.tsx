import type { Metadata } from "next"

import { ImageLightboxProvider } from "@/components/image-lightbox"
import { ConfirmProvider } from "@/components/confirm-provider"
import { SupportAuthProvider } from "@/app/(support)/support/_components/support-auth-provider"
import { ThemeProvider } from "@/components/theme-provider"
import { TooltipProvider } from "@/components/ui/tooltip"
import { Toaster } from "@/components/ui/sonner"
import { appFontVariables } from "@/lib/fonts"
import { AppI18nProvider } from "@/i18n/provider"

import "./support.css"
import "md-editor-rt/lib/style.css"

export const metadata: Metadata = {
  title: "AgentDesk Support",
  description: "AgentDesk Support Center",
}

export default function SupportRootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html lang="en-US" className={appFontVariables} suppressHydrationWarning>
      <body
        className="antialiased font-sans"
      >
        <AppI18nProvider>
          <ThemeProvider>
            <SupportAuthProvider>
              <ConfirmProvider>
                <ImageLightboxProvider>
                  <TooltipProvider>
                    {children}
                    <Toaster position="top-center" richColors />
                  </TooltipProvider>
                </ImageLightboxProvider>
              </ConfirmProvider>
            </SupportAuthProvider>
          </ThemeProvider>
        </AppI18nProvider>
      </body>
    </html>
  )
}
