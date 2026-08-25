import { getChatGPTUser } from "../../chatgpt-auth";
export async function GET(){const user=await getChatGPTUser();return Response.json({authenticated:Boolean(user),user:user?{displayName:user.displayName}:null,signIn:"/signin-with-chatgpt?return_to=/",signOut:"/signout-with-chatgpt?return_to=/"})}
