import {describe,expect,it} from 'vitest';
import {integrityLabel,rollbackLabel} from './status';

describe('audit status labels',()=>{
  it('never implies verification for absent integrity data',()=>expect(integrityLabel(undefined)).toBe('Unknown'));
  it('shows all rollback states',()=>expect(['available','partial','unavailable'].map(value=>rollbackLabel(value as 'available'|'partial'|'unavailable'))).toEqual(['Available','Partial','Unavailable']));
});
