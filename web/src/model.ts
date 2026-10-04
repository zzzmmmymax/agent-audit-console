import type {AuditEvent,EventFilters,Principal,RunFilters,RiskLevel} from './types';
export const PAGE_SIZE=100;
export const emptyRunFilters:RunFilters={q:'',agent:'',status:'',risk:'',integrity:'',repository:'',from:'',to:''};
export const emptyEventFilters:EventFilters={q:'',kind:'',risk:'',status:''};
export type ActionGroup={id:string;parentId:string;intent:string;status:string;risk:RiskLevel;startedAt:string;events:AuditEvent[];children:ActionGroup[]};
const riskWeight:Record<string,number>={unknown:0,low:1,medium:2,high:3};
export function groupActions(events:AuditEvent[]):ActionGroup[]{const groups=new Map<string,ActionGroup>();for(const event of events){const id=event.action_id||`event:${event.event_id}`;const current=groups.get(id)??{id,parentId:event.parent_action_id||'',intent:event.intent,status:event.action_status||'',risk:'unknown',startedAt:event.timestamp,events:[],children:[]};current.events.push(event);if(!current.intent)current.intent=event.intent;if(event.action_status)current.status=event.action_status;const next=event.risk?.level??'unknown';if(riskWeight[next]>riskWeight[current.risk])current.risk=next;if(event.timestamp<current.startedAt)current.startedAt=event.timestamp;groups.set(id,current)}const roots:ActionGroup[]=[];for(const group of groups.values()){const parent=group.parentId&&groups.get(group.parentId);if(parent)parent.children.push(group);else roots.push(group)}const sort=(items:ActionGroup[])=>items.sort((a,b)=>a.events[0].sequence-b.events[0].sequence).forEach(item=>sort(item.children));sort(roots);return roots}
export function sanitizeLog(value:unknown,max=60000){const text=String(value??'').replace(/\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])/g,'').replace(/[\u0000-\u0008\u000B\u000C\u000E-\u001F\u007F]/g,'');return text.length>max?`${text.slice(0,max)}\n\n[output truncated by console]`:text}
export function evidenceData(event:AuditEvent){return event.evidence?.data??{}}
export function displayCommand(event:AuditEvent){const value=evidenceData(event).command;return Array.isArray(value)?value.map(String).join(' '):typeof value==='string'?value:''}
export function isTestEvent(event:AuditEvent){const data=evidenceData(event);return data.category==='test'||data.test===true}
export function highestRisk(events:AuditEvent[]):RiskLevel{return events.reduce<RiskLevel>((level,event)=>{const next=event.risk?.level??'unknown';return riskWeight[next]>riskWeight[level]?next:level},'unknown')}
export function canRequestRollback(principal?:Principal){return principal?.role==='operator'||principal?.role==='admin'}
export function hasRedaction(value:string){return /\[REDACTED\]|\*{3,}/i.test(value)}
export function queryString(values:Record<string,string|number|undefined>){const params=new URLSearchParams();Object.entries(values).forEach(([key,value])=>{if(value!==''&&value!==undefined)params.set(key,String(value))});return params.toString()}
export function readState(search:string){const p=new URLSearchParams(search);return {run:p.get('run')??'',action:p.get('action')??'',event:p.get('event')??'',path:p.get('path')??''}}
export function terminalStatus(status:string){return ['completed','failed','cancelled','blocked'].includes(status.toLowerCase())}
