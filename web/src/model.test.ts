import {describe,expect,it} from 'vitest';
import {canRequestRollback,groupActions,queryString,readState,sanitizeLog,terminalStatus} from './model';
import type {AuditEvent} from './types';

function event(sequence:number,action:string,parent='',risk:'low'|'medium'|'high'='low'):AuditEvent{return {event_id:`event-${sequence}`,run_id:'run',action_id:action,parent_action_id:parent,action_status:sequence===3?'failed':'completed',sequence,kind:'command',intent:`action ${action}`,timestamp:`2026-01-01T00:00:0${sequence}Z`,risk:{level:risk},policy_decision:{status:'allowed'},evidence:{data:{command:['go','test','./...'],category:'test'}}}}

describe('audit investigation model',()=>{
 it('groups events by action and nests recorded parents',()=>{const grouped=groupActions([event(1,'parent'),event(2,'child','parent','high'),event(3,'parent')]);expect(grouped).toHaveLength(1);expect(grouped[0].events).toHaveLength(2);expect(grouped[0].children[0].id).toBe('child');expect(grouped[0].children[0].risk).toBe('high')});
 it('does not invent missing parents',()=>{const grouped=groupActions([event(1,'orphan','missing')]);expect(grouped[0].id).toBe('orphan');expect(grouped[0].children).toEqual([])});
 it('keeps filters server-readable and restores deep links',()=>{const query=queryString({q:'repo alpha',risk:'high',limit:50,empty:''});expect(query).toContain('q=repo+alpha');expect(query).toContain('risk=high');expect(query).not.toContain('empty');expect(readState('?run=r1&action=a1&event=e1')).toEqual({run:'r1',action:'a1',event:'e1',path:''})});
 it('removes ANSI and control characters and caps hostile logs',()=>{expect(sanitizeLog('\u001b[31mfail\u001b[0m\u0000',5)).toBe('fail');expect(sanitizeLog('123456',5)).toContain('truncated')});
 it('enforces rollback role and terminal live-refresh rules',()=>{expect(canRequestRollback({name:'v',role:'viewer'})).toBe(false);expect(canRequestRollback({name:'o',role:'operator'})).toBe(true);expect(terminalStatus('failed')).toBe(true);expect(terminalStatus('running')).toBe(false)});
 it('bounds a 10k-event timeline to the API page',()=>{const page=Array.from({length:100},(_,i)=>event(i+1,`a-${i}`));expect(groupActions(page)).toHaveLength(100);expect(page.length).toBeLessThan(10_000)});
});
