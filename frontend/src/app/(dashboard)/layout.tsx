"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Sidebar } from "@/components/layout/Sidebar";
import { FloatingActionButton } from "@/components/layout/FloatingActionButton";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const token = localStorage.getItem("token");
    if (!token) {
      router.replace("/login");
    } else {
      queueMicrotask(() => setReady(true));
    }
  }, [router]);

  if (!ready) {
    return (
      <div className="flex h-screen items-center justify-center" style={{ backgroundColor: "#f8f9ff" }}>
        <div className="size-8 animate-spin rounded-full border-2 border-[#3b6ef6] border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="flex h-screen" style={{ backgroundColor: "#f8f9ff" }}>
      <Sidebar />
      <main className="flex-1 overflow-auto pl-0">{children}</main>
      <FloatingActionButton />
    </div>
  );
}
