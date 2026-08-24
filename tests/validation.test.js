import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
test("all cloud stacks exist",()=>{for(const cloud of ["aws","azure","gcp","oci"]){assert.ok(fs.existsSync(`infra/${cloud}/main.tf`));assert.ok(fs.existsSync(`infra/${cloud}/variables.tf`));assert.ok(fs.existsSync(`infra/${cloud}/outputs.tf`))}});
test("container has a health check",()=>assert.match(fs.readFileSync("Dockerfile","utf8"),/HEALTHCHECK/));
test("admin password is not committed",()=>assert.doesNotMatch(fs.readFileSync(".env.example","utf8"),/ADMIN_PASSWORD=admin$/m));
