import { requireChatGPTUser } from "../chatgpt-auth";
import { isAdminEmail } from "../admin-auth";
import AdminDashboard from "./review-dashboard";
import Link from "next/link";

export const dynamic="force-dynamic";
export default async function AdminPage(){const user=await requireChatGPTUser("/admin");if(!isAdminEmail(user.email))return <main className="admin-denied"><p className="eyebrow">Restricted</p><h1>Administrator access required</h1><p>This account is signed in but is not authorized to moderate Flock Watcher.</p><Link href="/">Return to Flock Watcher</Link></main>;return <AdminDashboard userName={user.displayName}/>}
