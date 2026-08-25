import {readFile} from "node:fs/promises";

const programs=JSON.parse(await readFile(new URL("../data/eff-flock-programs.json",import.meta.url),"utf8"));
const coverage=JSON.parse(await readFile(new URL("../data/camera-coverage.json",import.meta.url),"utf8"));
const failures=[];
const check=(condition,message)=>{if(!condition)failures.push(message)};
const fresh=value=>Date.now()-new Date(value).getTime()<3*24*60*60*1000;

check(programs.recordCount===programs.programs.length,"EFF recordCount does not match the program array.");
check(programs.recordCount>=2000,"EFF Flock program count fell below the 2,000-record safety floor.");
check(programs.stateCount>=40,"EFF state coverage fell below 40 states.");
check(new Set(programs.programs.map(row=>row.id)).size===programs.programs.length,"EFF program IDs contain duplicates.");
check(programs.programs.every(row=>row.id&&row.agency&&row.state&&row.summary),"EFF records contain missing required fields.");
check(fresh(programs.retrievedAt),"EFF snapshot is older than three days.");
check(coverage.totalCameras>=100000,"National mapped-camera count fell below 100,000.");
check(coverage.identifiedFlock>=50000,"Identified Flock count fell below 50,000.");
check(coverage.statesRepresented>=50,"National coverage fell below 50 states or territories.");
check(coverage.states.reduce((sum,row)=>sum+row.total,0)===coverage.totalCameras,"State camera totals do not match the national total.");
check(coverage.states.reduce((sum,row)=>sum+row.flock,0)===coverage.identifiedFlock,"State Flock totals do not match the national total.");
check(fresh(coverage.retrievedAt),"National coverage snapshot is older than three days.");

if(failures.length){console.error(`Data validation failed:\n- ${failures.join("\n- ")}`);process.exit(1)}
console.log(`Validated ${coverage.totalCameras.toLocaleString()} mapped ALPRs, ${coverage.identifiedFlock.toLocaleString()} identified Flock cameras, and ${programs.recordCount.toLocaleString()} EFF programs.`);
