import coverage from "../../../data/camera-coverage.json";
export async function GET(){return Response.json(coverage,{headers:{"cache-control":"public, max-age=86400, stale-while-revalidate=604800","x-content-type-options":"nosniff"}})}
