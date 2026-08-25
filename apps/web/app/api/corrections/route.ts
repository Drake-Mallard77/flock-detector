import { env } from "cloudflare:workers";
import { getChatGPTUser } from "../../chatgpt-auth";
import { ensureSchema } from "../data/route";

export async function POST(request:Request){
 const user=await getChatGPTUser();
 if(!user)return Response.json({error:"Sign in with ChatGPT before requesting a correction.",signIn:"/signin-with-chatgpt?return_to=/"},{status:401});
 await ensureSchema();
 const body=await request.json() as {sightingId?:number;requestType?:string;explanation?:string;sourceUrl?:string};
 const sightingId=Number(body.sightingId),requestType=String(body.requestType??"correction"),explanation=String(body.explanation??"").trim().slice(0,2000),sourceUrl=String(body.sourceUrl??"").trim().slice(0,1000);
 if(!Number.isInteger(sightingId)||!new Set(["correction","challenge","removal"]).has(requestType)||explanation.length<20)return Response.json({error:"Provide a valid record, request type, and an explanation of at least 20 characters."},{status:400});
 if(sourceUrl){try{const u=new URL(sourceUrl);if(!["http:","https:"].includes(u.protocol))throw new Error()}catch{return Response.json({error:"Enter a valid public source URL."},{status:400})}}
 const record=await env.DB.prepare("SELECT id FROM sightings WHERE id=? AND status!='rejected'").bind(sightingId).first();if(!record)return Response.json({error:"Public record not found."},{status:404});
 const recent=await env.DB.prepare("SELECT COUNT(*) AS count FROM correction_requests WHERE submitted_by=? AND created_at>=datetime('now','-1 hour')").bind(user.email).first<{count:number}>();if(Number(recent?.count??0)>=5)return Response.json({error:"Correction-request limit reached. Try again later."},{status:429});
 const saved=await env.DB.prepare("INSERT INTO correction_requests (sighting_id,request_type,explanation,source_url,submitted_by) VALUES (?,?,?,?,?) RETURNING id").bind(sightingId,requestType,explanation,sourceUrl||null,user.email).first();
 return Response.json({ok:true,id:saved?.id},{status:201});
}
