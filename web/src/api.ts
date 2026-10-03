import {queryString} from './model';
import type {EventFilters,Principal,RollbackPreview,RunDetail,RunFilters,RunListResponse,VerifyResult} from './types';
export class ApiError extends Error{constructor(public status:number,message:string){super(message)}}
async function request<T>(path:string,init?:RequestInit):Promise<T>{const response=await fetch(path,init);if(!response.ok){let message=`HTTP ${response.status}`;try{const body=await response.json();message=body.error||message}catch{}throw new ApiError(response.status,message)}return response.json() as Promise<T>}
export function fetchRuns(filters:RunFilters,cursor=''){return request<RunListResponse>(`/api/run-overviews?${queryString({...filters,cursor,limit:50})}`)}
export function fetchRun(id:string,after:number,filters:EventFilters){return request<RunDetail>(`/api/runs/${encodeURIComponent(id)}?${queryString({...filters,after,limit:100})}`)}
export function fetchWhoami(){return request<Principal>('/api/whoami')}
export function verifyRun(id:string){return request<VerifyResult>(`/api/runs/${encodeURIComponent(id)}/verify`,{method:'POST'})}
export function fetchRollbackPreview(id:string){return request<RollbackPreview>(`/api/runs/${encodeURIComponent(id)}/rollback-preview`)}
export function requestRollback(id:string,snapshotId:string,reason:string){return request<{rollback_id:string;status:string}>(`/api/runs/${encodeURIComponent(id)}/rollback-requests`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({target_snapshot_id:snapshotId,reason})})}
