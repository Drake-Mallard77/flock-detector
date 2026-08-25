type OverpassElement={
  type:"node"|"way"|"relation";
  id:number;
  lat?:number;
  lon?:number;
  center?:{lat:number;lon:number};
  tags?:Record<string,string>;
};

const OVERPASS_URL="https://overpass-api.de/api/interpreter";
const MAX_RESULTS=4000;

function finite(value:string|null){const number=Number(value);return Number.isFinite(number)?number:null}

export async function GET(request:Request){
 const url=new URL(request.url),south=finite(url.searchParams.get("south")),west=finite(url.searchParams.get("west")),north=finite(url.searchParams.get("north")),east=finite(url.searchParams.get("east")),zoom=finite(url.searchParams.get("zoom"));
 if([south,west,north,east,zoom].some(value=>value===null))return Response.json({error:"A map viewport is required."},{status:400});
 if(south!<-90||north!>90||west!<-180||east!>180||south!>=north!||west!>=east!)return Response.json({error:"Invalid map viewport."},{status:400});
 if(zoom!<7||(north!-south!)>12||(east!-west!)>18)return Response.json({records:[],requiresZoom:true,message:"Zoom in to load individual public camera records."},{headers:{"cache-control":"public, max-age=300"}});
 const bbox=[south,west,north,east].map(value=>Number(value).toFixed(5)).join(",");
 const query=`[out:json][timeout:20];(nwr["man_made"="surveillance"]["surveillance:type"~"^(ALPR|alpr)$"](${bbox});nwr["man_made"="surveillance"]["camera:type"~"^(ALPR|alpr)$"](${bbox}););out center tags ${MAX_RESULTS};`;
 try{
  const response=await fetch(OVERPASS_URL,{method:"POST",headers:{"content-type":"application/x-www-form-urlencoded;charset=UTF-8","user-agent":"FlockWatcher/1.0 public-interest map"},body:new URLSearchParams({data:query}),signal:AbortSignal.timeout(24000)});
  if(!response.ok)throw new Error(`OpenStreetMap query returned ${response.status}`);
  const payload=await response.json() as {elements?:OverpassElement[]};
  const seen=new Set<string>(),records=(payload.elements??[]).flatMap(element=>{
   const latitude=element.lat??element.center?.lat,longitude=element.lon??element.center?.lon;
   if(!Number.isFinite(latitude)||!Number.isFinite(longitude))return [];
   const id=`osm-${element.type}-${element.id}`;if(seen.has(id))return [];seen.add(id);
   const tags=element.tags??{},vendor=tags.manufacturer||tags.brand||tags.operator||"Manufacturer not recorded",place=tags["addr:city"]||tags["is_in:city"]||tags.name||"Mapped ALPR";
   return [{id,city:place,state:tags["addr:state"]||"",latitude,longitude,status:"community-mapped",locationPrecision:"exact",agency:vendor,location:tags.description||"Publicly mapped automated license plate reader",sourceType:"openstreetmap",sourceUrl:`https://www.openstreetmap.org/${element.type}/${element.id}`,evidenceNote:`OpenStreetMap contributor record${tags.survey_date?` · surveyed ${tags.survey_date}`:""}. Vendor identification may be incomplete.`}];
  });
  return Response.json({records,truncated:records.length>=MAX_RESULTS,source:"OpenStreetMap contributors",license:"ODbL 1.0",retrievedAt:new Date().toISOString()},{headers:{"cache-control":"public, max-age=3600, stale-while-revalidate=86400","x-content-type-options":"nosniff"}});
 }catch(error){return Response.json({error:error instanceof Error?error.message:"OpenStreetMap data is temporarily unavailable.",records:[]},{status:502,headers:{"cache-control":"public, max-age=60"}})}
}
