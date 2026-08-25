import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Flock Watcher — Public infrastructure tracker",
  description: "Search documented automated license plate reader locations, review evidence, submit sightings, and track public contracts.",
  metadataBase: new URL("https://flock-watch.simwes07.chatgpt.site"),
  openGraph: {
    title: "Flock Watcher — Public infrastructure tracker",
    description: "Search reported camera locations, review evidence, and track public contracts.",
    images: [{ url: "/og.png", width: 1200, height: 630, alt: "Flock Watcher public infrastructure tracker" }],
  },
  twitter: {
    card: "summary_large_image",
    title: "Flock Watcher — Public infrastructure tracker",
    description: "Search reported camera locations, review evidence, and track public contracts.",
    images: ["/og.png"],
  },
  icons: {
    icon: "/favicon.svg",
    shortcut: "/favicon.svg",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="antialiased">{children}</body>
    </html>
  );
}
