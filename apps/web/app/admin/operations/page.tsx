import Link from "next/link";
import { requireChatGPTUser } from "../../chatgpt-auth";
import { isAdminEmail } from "../../admin-auth";
import OperationsDashboard from "./operations-dashboard";
export const dynamic="force-dynamic";
export default async function OperationsPage(){const user=await requireChatGPTUser("/admin/operations");if(!isAdminEmail(user.email))return <main className="admin-denied"><h1>Administrator access required</h1><p>This operational console is restricted to authorized Flock Watcher administrators.</p><Link href="/">Return to the public map</Link></main>;return <div className="admin-shell"><header className="admin-top"><Link href="/" className="brand"><span className="brand-mark">FW</span><span>Flock Watcher<small>Operations console</small></span></Link><div><Link href="/admin">Sightings</Link><Link href="/admin/sources">Sources</Link><Link href="/admin/corrections">Corrections</Link></div></header><main><OperationsDashboard/></main></div>}
