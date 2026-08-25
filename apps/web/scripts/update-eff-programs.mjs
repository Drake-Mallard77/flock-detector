import {mkdir,writeFile} from "node:fs/promises";

const SOURCE="https://atlasofsurveillance.org/download.csv";
const OUTPUT=new URL("../data/eff-flock-programs.json",import.meta.url);

function rows(text){
 const result=[];let row=[],field="",quoted=false;
 for(let i=0;i<text.length;i++){const char=text[i];if(quoted){if(char==='"'&&text[i+1]==='"'){field+='"';i++}else if(char==='"')quoted=false;else field+=char}else if(char==='"')quoted=true;else if(char===','){row.push(field);field=""}else if(char==='\n'){row.push(field.replace(/\r$/,""));result.push(row);row=[];field=""}else field+=char}
 if(field||row.length){row.push(field);result.push(row)}return result;
}

const response=await fetch(SOURCE,{signal:AbortSignal.timeout(60000)});
if(!response.ok)throw new Error(`EFF Atlas download failed: ${response.status}`);
const parsed=rows(await response.text()),headers=parsed.shift(),index=Object.fromEntries(headers.map((name,i)=>[name.replace(/^\uFEFF/,""),i]));
const pick=(row,name)=>row[index[name]]?.trim()??"";
const programs=parsed.filter(row=>pick(row,"Technology")==="Automated License Plate Readers"&&/flock/i.test(`${pick(row,"Vendor")} ${pick(row,"Summary")}`)).map(row=>({
 id:pick(row,"AOSNUMBER"),city:pick(row,"City"),county:pick(row,"County"),state:pick(row,"State"),agency:pick(row,"Agency"),agencyType:pick(row,"Type of LEA"),jurisdictionType:pick(row,"Type of Juris"),vendor:pick(row,"Vendor")||"Flock Safety mentioned in source",summary:pick(row,"Summary"),sourceUrl:pick(row,"Link 1"),sourceDate:pick(row,"Link 1 Date")
})).sort((a,b)=>a.state.localeCompare(b.state)||a.city.localeCompare(b.city)||a.agency.localeCompare(b.agency));
const artifact={source:SOURCE,sourceName:"Electronic Frontier Foundation Atlas of Surveillance",license:"CC BY",retrievedAt:new Date().toISOString(),recordCount:programs.length,stateCount:new Set(programs.map(row=>row.state).filter(Boolean)).size,programs};
await mkdir(new URL("../data/",import.meta.url),{recursive:true});
await writeFile(OUTPUT,JSON.stringify(artifact));
console.log(`Saved ${artifact.recordCount} Flock-related ALPR programs across ${artifact.stateCount} states.`);
