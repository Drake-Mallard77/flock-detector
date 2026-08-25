import Link from "next/link";
import { requireChatGPTUser } from "../../chatgpt-auth";
import { isAdminEmail } from "../../admin-auth";
import SourceDashboard from "./source-dashboard";
export const dynamic="force-dynamic";
export default async function SourcesPage(){const user=await requireChatGPTUser("/admin/sources");if(!isAdminEmail(user.email))return <main className="admin-denied"><h1>Administrator access required</h1><p>This source queue is restricted to authorized Flock Watcher moderators.</p><Link href="/">Return to the public map</Link></main>;return <div className="admin-shell"><header className="admin-top"><Link href="/" className="brand"><span className="brand-mark">FW</span><span>Flock Watcher<small>Source operations</small></span></Link><div><Link href="/admin">Sightings</Link><Link href="/admin/operations">Operations</Link><Link href="/admin/corrections">Corrections</Link><a href="/api/admin/export">Download backup</a></div></header><main><SourceDashboard/></main></div>}
