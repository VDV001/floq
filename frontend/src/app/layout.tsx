import type { Metadata } from "next";
import { Manrope } from "next/font/google";
import "./globals.css";
import { NotificationProvider } from "@/components/notifications/NotificationProvider";

const manrope = Manrope({
  subsets: ["latin", "cyrillic"],
});

export const metadata: Metadata = {
  title: "Floq — AI Sales Assistant",
  description: "AI-powered inbound sales assistant for small B2B businesses",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="ru" className={`${manrope.className} h-full antialiased`}>
      <body className="min-h-full flex flex-col">
        <NotificationProvider>{children}</NotificationProvider>
      </body>
    </html>
  );
}
