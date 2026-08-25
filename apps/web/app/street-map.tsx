"use client";
import { useEffect,useRef } from "react";
import type { Map as LeafletMap,LayerGroup } from "leaflet";

export type MapSighting={id:string|number;city:string;state:string;latitude:number;longitude:number;status:string;locationPrecision:string};
export type MapViewport={south:number;west:number;north:number;east:number;zoom:number};
export default function StreetMap({sightings,onSelect,onViewportChange}:{sightings:MapSighting[];onSelect:(s:MapSighting)=>void;onViewportChange?:(viewport:MapViewport)=>void}){const host=useRef<HTMLDivElement>(null),map=useRef<LeafletMap|null>(null),layers=useRef<LayerGroup|null>(null),latest=useRef(sightings),select=useRef(onSelect),viewport=useRef(onViewportChange);
 useEffect(()=>{let disposed=false;async function start(){const L=await import("leaflet");if(disposed||!host.current||map.current)return;const instance=L.map(host.current,{zoomControl:true,minZoom:3,maxZoom:18}).setView([38.5,-96],4);L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png",{attribution:'&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',maxZoom:19}).addTo(instance);const group=L.layerGroup().addTo(instance);map.current=instance;layers.current=group;
  function render(){group.clearLayers();const items=latest.current;if(instance.getZoom()<=5){const buckets=new Map<string,MapSighting[]>();for(const item of items){const key=`${Math.round(item.latitude/5)}:${Math.round(item.longitude/5)}`;buckets.set(key,[...(buckets.get(key)??[]),item])}for(const bucket of buckets.values()){if(bucket.length>1){const lat=bucket.reduce((a,s)=>a+s.latitude,0)/bucket.length,lon=bucket.reduce((a,s)=>a+s.longitude,0)/bucket.length;const marker=L.marker([lat,lon],{icon:L.divIcon({className:"",html:`<span class="street-cluster">${bucket.length}</span>`,iconSize:[38,38],iconAnchor:[19,19]})}).addTo(group);marker.on("click",()=>instance.setView([lat,lon],8));continue}add(bucket[0])}}else for(const item of items)add(item)}
  function add(item:MapSighting){const precision=["exact","approximate","jurisdiction"].includes(item.locationPrecision)?item.locationPrecision:"approximate";const status=["verified","documented","submitted","disputed"].includes(item.status)?item.status:"submitted";const marker=L.marker([item.latitude,item.longitude],{icon:L.divIcon({className:"",html:`<span class="street-pin ${precision} ${status}"></span>`,iconSize:[30,30],iconAnchor:[15,15]})}).addTo(group);const tip=document.createElement("span");tip.textContent=`${item.city}, ${item.state} · ${precision}`;marker.bindTooltip(tip);marker.on("click",()=>select.current(item))}
  function reportViewport(){const bounds=instance.getBounds();viewport.current?.({south:bounds.getSouth(),west:bounds.getWest(),north:bounds.getNorth(),east:bounds.getEast(),zoom:instance.getZoom()})}
  instance.on("zoomend",render);instance.on("moveend",reportViewport);render();reportViewport()}
 start();return()=>{disposed=true;if(map.current){map.current.remove();map.current=null;layers.current=null}}},[]);
 useEffect(()=>{latest.current=sightings;if(map.current)map.current.fire("zoomend")},[sightings]);
 useEffect(()=>{select.current=onSelect},[onSelect]);
 useEffect(()=>{viewport.current=onViewportChange},[onViewportChange]);
 return <div className="street-map" ref={host} aria-label={`Interactive street map with ${sightings.length} records`}/>}
