export type IntegrityStatus='verified'|'broken'|'unknown'|undefined;
export type RollbackStatus='available'|'partial'|'unavailable'|'unknown'|undefined;

export function integrityLabel(status:IntegrityStatus){return status==='verified'?'Verified':status==='broken'?'Broken':'Unknown'}
export function rollbackLabel(status:RollbackStatus){return status==='available'?'Available':status==='partial'?'Partial':status==='unavailable'?'Unavailable':'Unknown'}
