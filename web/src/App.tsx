import { Navigate, Route, Routes } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "./api";
import Login from "./pages/Login";
import Library from "./pages/Library";
import GameDetail from "./pages/GameDetail";
import AddGame from "./pages/AddGame";

export default function App() {
  const me = useQuery({
    queryKey: ["me"],
    queryFn: api.me,
  });

  if (me.isLoading) {
    return (
      <div className="grid min-h-screen place-items-center text-[#9aa3b2]">
        Loading…
      </div>
    );
  }

  if (me.isError) {
    return (
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    );
  }

  return (
    <div className="min-h-screen">
      <Routes>
        <Route path="/" element={<Library igdb={!!me.data?.igdb_configured} pricecharting={!!me.data?.pricecharting_configured} />} />
        <Route path="/add" element={<AddGame igdb={me.data.igdb_configured} />} />
        <Route path="/game/:id" element={<GameDetail />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </div>
  );
}
