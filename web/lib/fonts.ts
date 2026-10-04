import { IBM_Plex_Mono, Inter } from "next/font/google"

export const appSans = Inter({
  variable: "--font-app-sans",
  subsets: ["latin", "latin-ext", "cyrillic", "cyrillic-ext"],
  display: "swap",
})

export const appMono = IBM_Plex_Mono({
  variable: "--font-app-mono",
  subsets: ["latin", "latin-ext", "cyrillic", "cyrillic-ext"],
  weight: ["400", "500", "600"],
  display: "swap",
})

export const appFontVariables = `${appSans.variable} ${appMono.variable}`