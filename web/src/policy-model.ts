import type {PolicyExplain,PolicyRule} from './types';

export function policyChangeLabel(value:PolicyExplain){
 if(!value.recorded_available)return 'Recorded policy unavailable';
 return value.policy_changed?'Policy changed since this action':'Policy unchanged';
}

export function orderedRules(rules:PolicyRule[]){
 return [...rules].sort((left,right)=>right.priority-left.priority||right.source.localeCompare(left.source)||left.id.localeCompare(right.id));
}

export function canSimulatePolicy(){return true}
