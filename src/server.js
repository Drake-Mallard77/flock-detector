import crypto from "node:crypto";
import path from "node:path";
import { fileURLToPath } from "node:url";
import express from "express";
import helmet from "helmet";
import { migrate, pool } from "./db.js";

const app = express(), port = Number(process.env.PORT || 8080), root = path.dirname(fileURLToPath(import.meta.url));
if (process.env.TRUST_PROXY) app.set("trust proxy", Number(process.env.TRUST_PROXY) || 1);
app.use(helmet({ contentSecurityPolicy: { directives: { defaultSrc:["'self'"], imgSrc:["'self'","data:"], styleSrc:["'self'","'unsafe-inline'"], scriptSrc:["'self'"] } } }));
app.use(express.json({ limit:"64kb" }));

function text(value, max=1000){ return String(value ?? "").trim().slice(0,max); }
function validUrl(value){ try { const u=new URL(value); return ["http:","https:"].includes(u.protocol); } catch { return false; } }
function safeEqual(left,right){const a=Buffer.from(left||""),b=Buffer.from(right||"");return a.length===b.length&&crypto.timingSafeEqual(a,b)}
function basicAuth(req,res,next){
  const auth=req.headers.authorization||"", expectedUser=process.env.ADMIN_USERNAME||"admin", expectedPass=process.env.ADMIN_PASSWORD||"";
  if(auth.startsWith("Basic ")){const [user,pass]=Buffer.from(auth.slice(6),"base64").toString().split(":");if(safeEqual(user,expectedUser)&&safeEqual(pass,expectedPass)){req.adminUser=user;return next();}}
  res.set("WWW-Authenticate",'Basic realm="FlockWatch Review Desk"');return res.status(401).send("Authentication required");
}

app.get("/health", async (_req,res)=>{try{await pool.query("SELECT 1");res.json({status:"ok"})}catch{res.status(503).json({status:"unavailable"})}});
app.get("/api/deployments", async (req,res,next)=>{try{const q=text(req.query.q,100),state=text(req.query.state,2).toUpperCase(),status=text(req.query.status,30),where=["lifecycle <> 'removed'"],values=[];if(q){values.push(`%${q}%`);where.push(`(city ILIKE $${values.length} OR state ILIKE $${values.length} OR agency ILIKE $${values.length})`)}if(state){values.push(state);where.push(`state=$${values.length}`)}if(status){values.push(status);where.push(`status=$${values.length}`)}const {rows}=await pool.query(`SELECT id,city,state,agency,status,cameras,evidence,source_url AS source,latitude,longitude,map_x AS x,map_y AS y,reviewed_at AS updated,lifecycle FROM deployments WHERE ${where.join(" AND ")} ORDER BY state,city LIMIT 1000`,values);res.json({deployments:rows,meta:{count:rows.length,generatedAt:new Date().toISOString()}})}catch(e){next(e)}});
app.post("/api/submissions",async(req,res,next)=>{try{const city=text(req.body.city,100),state=text(req.body.state,2).toUpperCase(),location=text(req.body.location,300),url=text(req.body.evidenceUrl,1000),notes=text(req.body.notes,2000);if(!city||!/^[A-Z]{2}$/.test(state)||!location||!validUrl(url)||!notes)return res.status(400).json({error:"Valid city, state, location, evidence URL and notes are required."});await pool.query("INSERT INTO submissions(city,state,location,evidence_url,notes) VALUES($1,$2,$3,$4,$5)",[city,state,location,url,notes]);res.status(201).json({accepted:true,status:"pending"})}catch(e){next(e)}});
app.post("/api/corrections",async(req,res,next)=>{try{const id=Number(req.body.deploymentId),kind=text(req.body.correctionType,50),details=text(req.body.details,2000),url=text(req.body.evidenceUrl,1000);if(!Number.isInteger(id)||!kind||!details||(url&&!validUrl(url)))return res.status(400).json({error:"Valid correction details are required."});await pool.query("INSERT INTO corrections(deployment_id,correction_type,details,evidence_url) VALUES($1,$2,$3,$4)",[id,kind,details,url||null]);res.status(201).json({accepted:true,status:"pending"})}catch(e){next(e)}});
app.use("/admin",basicAuth,express.static(path.join(root,"../public/admin"),{index:"index.html"}));
app.get("/api/admin/moderation",basicAuth,async(_req,res,next)=>{try{const [s,c,j,n]=await Promise.all([pool.query("SELECT * FROM submissions ORDER BY status='pending' DESC,created_at DESC LIMIT 100"),pool.query("SELECT c.*,d.city,d.state,d.agency FROM corrections c LEFT JOIN deployments d ON d.id=c.deployment_id ORDER BY c.status='pending' DESC,c.created_at DESC LIMIT 100"),pool.query("SELECT * FROM ingest_jobs ORDER BY created_at DESC LIMIT 50"),pool.query("SELECT (SELECT COUNT(*) FROM deployments)::int deployments,(SELECT COUNT(*) FROM submissions WHERE status='pending')::int submissions,(SELECT COUNT(*) FROM corrections WHERE status='pending')::int corrections")]);res.json({submissions:s.rows,corrections:c.rows,jobs:j.rows,counts:n.rows[0]})}catch(e){next(e)}});
app.patch("/api/admin/moderation",basicAuth,async(req,res,next)=>{try{const kind=text(req.body.kind,20),id=Number(req.body.id),action=text(req.body.action,20),note=text(req.body.note,500),table=kind==="submission"?"submissions":kind==="correction"?"corrections":null;if(!table||!Number.isInteger(id)||!["approved","rejected"].includes(action))return res.status(400).json({error:"Invalid moderation action."});await pool.query(`UPDATE ${table} SET status=$1,reviewer_note=$2,reviewed_by=$3,reviewed_at=NOW() WHERE id=$4`,[action,note,req.adminUser,id]);if(kind==="submission"&&action==="approved"){const {rows}=await pool.query("SELECT * FROM submissions WHERE id=$1",[id]);if(rows[0]){const s=rows[0],x={CA:15,CO:36,IL:64,RI:91}[s.state]||76;await pool.query("INSERT INTO deployments(city,state,agency,status,evidence,source_url,latitude,longitude,map_x,map_y,reviewed_at) VALUES($1,$2,'Community-reported sighting','Under review',$3,$4,$5,$6,$7,50,NOW())",[s.city,s.state,s.notes,s.evidence_url,s.latitude,s.longitude,x])}}res.json({updated:true})}catch(e){next(e)}});
app.post("/api/admin/ingest",basicAuth,async(req,res,next)=>{try{const url=text(req.body.sourceUrl,1000),type=text(req.body.sourceType,100),agency=text(req.body.agency,200),notes=text(req.body.notes,1000);if(!validUrl(url)||!type)return res.status(400).json({error:"Source URL and type are required."});const {rows}=await pool.query("INSERT INTO ingest_jobs(source_url,source_type,agency,notes,created_by) VALUES($1,$2,$3,$4,$5) RETURNING id",[url,type,agency||null,notes||null,req.adminUser]);res.status(201).json({queued:true,id:rows[0].id})}catch(e){next(e)}});
app.use(express.static(path.join(root,"../public"),{extensions:["html"]}));
app.use((err,_req,res,_next)=>{console.error(err);res.status(500).json({error:"Internal server error"})});

await migrate();
app.listen(port,"0.0.0.0",()=>console.log(`FlockWatch listening on ${port}`));
