"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Sparkles, User, Mail, Lock, Eye, EyeOff, ArrowRight, Info } from "lucide-react";
import { api } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [fullName, setFullName] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  // Simple password strength (0-4)
  const passwordStrength = (() => {
    if (!password) return 0;
    let s = 0;
    if (password.length >= 6) s++;
    if (password.length >= 8) s++;
    if (/[A-Z]/.test(password) && /[a-z]/.test(password)) s++;
    if (/\d/.test(password) || /[^a-zA-Z0-9]/.test(password)) s++;
    return s;
  })();

  const strengthLabel = ["", "Слабый", "Средний", "Надежный", "Отличный"][passwordStrength] || "";
  const strengthColor = ["", "text-[#ba1a1a]", "text-[#f59e0b]", "text-[#3e3fcc]", "text-green-600"][passwordStrength] || "";

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      const { token, refresh_token } =
        mode === "login"
          ? await api.login(email, password)
          : await api.register(email, password, fullName);
      localStorage.setItem("token", token);
      localStorage.setItem("refresh_token", refresh_token);
      router.push("/inbox");
    } catch {
      setError(
        mode === "login"
          ? "Неверный email или пароль"
          : "Ошибка регистрации. Возможно, email уже занят."
      );
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="relative flex min-h-screen flex-col text-[#0d1c2e]" style={{ backgroundColor: "#f8f9ff", backgroundImage: "radial-gradient(at 0% 0%, rgba(37,99,235,0.05) 0px, transparent 50%), radial-gradient(at 100% 100%, rgba(62,63,204,0.05) 0px, transparent 50%)" }}>
      {/* Background decoration */}
      <div className="pointer-events-none fixed inset-0 -z-10 overflow-hidden">
        <div className="absolute -right-[10%] -top-[20%] size-[60%] rounded-full bg-[#dbe1ff]/30 blur-[120px]" />
        <div className="absolute -bottom-[10%] -left-[10%] size-[40%] rounded-full bg-[#e1e0ff]/40 blur-[100px]" />
      </div>

      {/* Logo */}
      <header className="flex w-full items-center justify-center py-12">
        <div className="flex items-center gap-2">
          <Sparkles className="size-10 text-[#3b6ef6]" />
          <h1 className="text-3xl font-extrabold tracking-tight">Floq</h1>
        </div>
      </header>

      {/* Main */}
      <main className="flex flex-grow items-center justify-center px-4 pb-24">
        <div className="w-full max-w-md rounded-xl bg-white p-8 shadow-[0_12px_40px_rgba(13,28,46,0.06)] ring-1 ring-[#c3c6d7]/15 md:p-10">
          {/* Title */}
          <div className="mb-10 text-center">
            <h2 className="mb-3 text-3xl font-bold tracking-tight">
              {mode === "login" ? "Вход в Floq" : "Создать аккаунт"}
            </h2>
            <p className="leading-relaxed text-[#434655]">
              {mode === "login"
                ? "Ваш ИИ-ассистент по продажам"
                : "Начните продавать быстрее с помощью ИИ"}
            </p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-6">
            {/* Name (register only) */}
            {mode === "register" && (
              <div className="space-y-2">
                <label className="ml-1 block text-sm font-semibold">Имя</label>
                <div className="relative">
                  <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-4">
                    <User className="size-5 text-[#737686]" />
                  </div>
                  <input
                    type="text"
                    placeholder="Иван Иванов"
                    value={fullName}
                    onChange={(e) => setFullName(e.target.value)}
                    required
                    className="w-full rounded-lg border-transparent bg-[#eff4ff] py-3 pl-11 pr-4 text-sm transition-all duration-200 placeholder:text-[#737686] focus:border-[#004ac6] focus:ring-2 focus:ring-[#dbe1ff]"
                  />
                </div>
              </div>
            )}

            {/* Email */}
            <div className="space-y-2">
              <label className="ml-1 block text-sm font-semibold">Email</label>
              <div className="relative">
                {mode === "register" && (
                  <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-4">
                    <Mail className="size-5 text-[#737686]" />
                  </div>
                )}
                <input
                  type="email"
                  placeholder="name@company.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  className={`w-full rounded-lg border ${mode === "register" ? "border-transparent bg-[#eff4ff] pl-11" : "border-[#c3c6d7]/20 bg-white pl-4"} py-3 pr-4 text-sm transition-all duration-200 outline-none placeholder:text-[#737686] focus:border-[#004ac6] focus:ring-2 focus:ring-[#dbe1ff]`}
                />
              </div>
            </div>

            {/* Password */}
            <div className="space-y-2">
              <div className="flex items-center justify-between px-1">
                <label className="text-sm font-semibold">Пароль</label>
                {mode === "login" && (
                  <button
                    type="button"
                    className="text-sm font-medium text-[#004ac6] transition-all hover:underline"
                  >
                    Забыли пароль?
                  </button>
                )}
              </div>
              <div className="relative">
                {mode === "register" && (
                  <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-4">
                    <Lock className="size-5 text-[#737686]" />
                  </div>
                )}
                <input
                  type={showPassword ? "text" : "password"}
                  placeholder="••••••••"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  minLength={6}
                  className={`w-full rounded-lg border ${mode === "register" ? "border-transparent bg-[#eff4ff] pl-11" : "border-[#c3c6d7]/20 bg-white pl-4"} py-3 pr-11 text-sm transition-all duration-200 outline-none placeholder:text-[#737686] focus:border-[#004ac6] focus:ring-2 focus:ring-[#dbe1ff]`}
                />
                <button
                  type="button"
                  onClick={(e) => { e.preventDefault(); e.stopPropagation(); setShowPassword(!showPassword); }}
                  className="absolute inset-y-0 right-0 z-10 flex items-center pr-4 text-[#737686] transition-colors hover:text-[#004ac6]"
                >
                  {showPassword ? <EyeOff className="size-5" /> : <Eye className="size-5" />}
                </button>
              </div>

              {/* Password strength (register only) */}
              {mode === "register" && password && (
                <div className="px-1 pt-1">
                  <div className="mb-2 flex h-1 gap-1">
                    {[1, 2, 3, 4].map((i) => (
                      <div
                        key={i}
                        className={`h-full flex-1 rounded-full ${i <= passwordStrength ? "bg-[#004ac6]" : "bg-[#c3c6d7]/30"}`}
                      />
                    ))}
                  </div>
                  <div className="flex items-center gap-1.5">
                    <Info className={`size-3.5 ${strengthColor}`} />
                    <span className={`text-[11px] font-medium uppercase tracking-wider ${strengthColor}`}>
                      {strengthLabel} пароль
                    </span>
                  </div>
                </div>
              )}
            </div>

            {/* AI Insight (register only) */}
            {mode === "register" && (
              <div className="flex items-start gap-3 rounded-lg bg-[#e1e0ff]/40 p-4 shadow-[0_0_20px_rgba(62,63,204,0.1)]">
                <Sparkles className="mt-0.5 size-5 shrink-0 text-[#3e3fcc]" />
                <p className="text-xs leading-relaxed text-[#2f2ebe]">
                  Ваш аккаунт будет автоматически настроен на поиск лидов в
                  вашем регионе после завершения регистрации.
                </p>
              </div>
            )}

            {/* Error */}
            {error && <p className="text-sm text-[#ba1a1a]">{error}</p>}

            {/* Submit */}
            <button
              type="submit"
              disabled={loading}
              className="flex w-full items-center justify-center gap-2 rounded-lg bg-gradient-to-br from-[#004ac6] to-[#2563eb] py-3.5 font-semibold text-white shadow-lg transition-all hover:opacity-90 active:scale-[0.98] disabled:opacity-50"
            >
              {loading ? (
                "..."
              ) : mode === "login" ? (
                "Войти"
              ) : (
                <>
                  Создать аккаунт
                  <ArrowRight className="size-5" />
                </>
              )}
            </button>
          </form>

          {/* Login mode: social divider + Google */}
          {mode === "login" && (
            <>
              <div className="relative my-10">
                <div className="absolute inset-0 flex items-center">
                  <div className="w-full border-t border-[#c3c6d7]/15" />
                </div>
                <div className="relative flex justify-center text-xs uppercase tracking-widest">
                  <span className="bg-white px-4 font-medium text-[#737686]">
                    или войти через
                  </span>
                </div>
              </div>
              <button className="flex w-full items-center justify-center gap-3 rounded-lg border border-[#c3c6d7]/10 bg-[#eff4ff] py-3 font-medium text-[#434655] transition-colors duration-200 hover:bg-[#e6eeff]">
                <svg width="20" height="20" viewBox="0 0 24 24">
                  <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4" />
                  <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853" />
                  <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l3.66-2.84z" fill="#FBBC05" />
                  <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335" />
                </svg>
                Google
              </button>
            </>
          )}

          {/* Register mode: legal text */}
          {mode === "register" && (
            <div className="mt-8 text-center">
              <p className="px-4 text-xs leading-relaxed text-[#434655]">
                Регистрируясь, вы соглашаетесь с{" "}
                <a className="font-medium text-[#004ac6] hover:underline" href="#">
                  Условиями использования
                </a>{" "}
                и{" "}
                <a className="font-medium text-[#004ac6] hover:underline" href="#">
                  Политикой конфиденциальности
                </a>
              </p>
            </div>
          )}

          {/* Toggle mode */}
          <div className={`text-center ${mode === "register" ? "mt-8 border-t border-[#c3c6d7]/10 pt-8" : "mt-10"}`}>
            <p className="text-sm text-[#434655]">
              {mode === "login" ? "Нет аккаунта?" : "Уже есть аккаунт?"}
              <button
                type="button"
                onClick={() => {
                  setMode(mode === "login" ? "register" : "login");
                  setError("");
                }}
                className="ml-1 font-bold text-[#004ac6] hover:underline"
              >
                {mode === "login" ? "Зарегистрироваться" : "Войти"}
              </button>
            </p>
          </div>
        </div>
      </main>

      {/* Footer */}
      <footer className="flex w-full flex-col items-center gap-4 pb-8 text-sm">
        <div className="flex gap-6 text-slate-400">
          <a className="transition-colors hover:text-slate-600" href="#">Политика</a>
          <a className="transition-colors hover:text-slate-600" href="#">Условия</a>
          <a className="transition-colors hover:text-slate-600" href="#">Помощь</a>
        </div>
        <p className="text-slate-400">© 2026 Floq Technologies</p>
      </footer>
    </div>
  );
}
