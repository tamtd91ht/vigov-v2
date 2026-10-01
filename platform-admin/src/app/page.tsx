import { redirect } from "next/navigation";

/** The console opens on its first wave-1 section (ADR 0048 §01/10 #4). */
export default function Home() {
  redirect("/xa");
}
