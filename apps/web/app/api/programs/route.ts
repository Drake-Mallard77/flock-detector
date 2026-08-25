import dataset from "../../../data/eff-flock-programs.json";

export async function GET(){
 return Response.json(dataset,{headers:{"cache-control":"public, max-age=86400, stale-while-revalidate=604800","x-content-type-options":"nosniff"}});
}
