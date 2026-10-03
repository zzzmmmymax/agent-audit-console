import {describe,expect,it} from 'vitest';
import {canSimulatePolicy,orderedRules,policyChangeLabel} from './policy-model';

describe('policy inspector model',()=>{
 it('keeps read-only simulation available to viewers',()=>expect(canSimulatePolicy()).toBe(true));
 it('distinguishes recorded, changed, and unavailable policy',()=>{
  const base:any={recorded:{},current:{},re_evaluated:true};
  expect(policyChangeLabel({...base,recorded_available:false,policy_changed:false})).toContain('unavailable');
  expect(policyChangeLabel({...base,recorded_available:true,policy_changed:true})).toContain('changed');
  expect(policyChangeLabel({...base,recorded_available:true,policy_changed:false})).toBe('Policy unchanged');
 });
 it('orders rule details deterministically',()=>expect(orderedRules([{id:'b',risk:'low',decision:'allow',priority:1,source:'project'},{id:'a',risk:'high',decision:'deny',priority:2,source:'team'}])[0].id).toBe('a'));
});
