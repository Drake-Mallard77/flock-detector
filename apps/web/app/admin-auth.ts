import { env } from "cloudflare:workers";
import { getChatGPTUser } from "./chatgpt-auth";
function configuredAdmins(){const raw=String((env as unknown as Record<string,unknown>).ADMIN_EMAILS??"");return new Set(raw.split(",").map(v=>v.trim().toLowerCase()).filter(Boolean))}
export function isAdminEmail(email:string){const admins=configuredAdmins();return admins.size>0&&admins.has(email.trim().toLowerCase())}
export async function getAdminUser(){const user=await getChatGPTUser();return user&&isAdminEmail(user.email)?user:null}
