import Link from "next/link";
import { requireChatGPTUser } from "../../chatgpt-auth";
import { isAdminEmail } from "../../admin-auth";
import CorrectionDashboard from "./correction-dashboard";
export const dynamic="force-dynamic";
export default async function CorrectionsPage(){const user=await requireChatGPTUser("/admin/corrections");if(!isAdminEmail(user.email))return <main className="admin-denied"><h1>Administrator access required</h1><p>This queue is restricted to authorized Flock Watcher moderators.</p><Link href="/">Return to the public map</Link></main>;return <div className="admin-shell"><header className="admin-top"><Link href="/" className="brand"><span className="brand-mark">FW</span><span>Flock Watcher<small>Correction operations</small></span></Link><div><Link href="/admin">Sightings</Link><Link href="/admin/operations">Operations</Link><a href="/api/admin/export">Download backup</a><a href="/signout-with-chatgpt?return_to=/">Sign out</a></div></header><main><CorrectionDashboard/></main></div>}
