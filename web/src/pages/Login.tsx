import { FormEvent, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { api } from "../api";

export default function Login() {
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const nav = useNavigate();
  const qc = useQueryClient();

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api.login(password.trim());
      const me = await api.me();
      qc.setQueryData(["me"], me);
      nav("/");
    } catch (err) {
      const status = (err as Error & { status?: number }).status;
      if (status === 401) {
        setError("Wrong password.");
      } else if (err instanceof TypeError) {
        setError("Can't reach the server. Is it running?");
      } else {
        setError(err instanceof Error ? err.message : "Could not sign in.");
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto flex min-h-screen max-w-md flex-col justify-center px-6">
      <h1 className="text-3xl font-semibold tracking-tight">game-db</h1>
      <p className="mt-2 text-[#9aa3b2]">Physical library, on your hardware.</p>
      <form onSubmit={onSubmit} className="mt-8 space-y-4">
        <label className="block text-sm text-[#9aa3b2]">
          Password
          <input
            type="password"
            autoFocus
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="mt-1 w-full rounded-lg border border-[#2a2e38] bg-[#16181f] px-3 py-2 text-[#e8eaef] outline-none focus:border-[#e2b14a]"
          />
        </label>
        {error && <p className="text-sm text-red-400">{error}</p>}
        <button
          type="submit"
          disabled={busy || !password}
          className="w-full rounded-lg bg-[#e2b14a] px-3 py-2 font-medium text-[#111] disabled:opacity-50"
        >
          Sign in
        </button>
      </form>
    </div>
  );
}
