// 极简 router（不引外部库）
import { getContext, setContext } from 'svelte';

const KEY = 'linkall.router';
type Nav = (page: string) => void;

export const setRouter = (nav: Nav) => setContext(KEY, nav);
export const goto = (page: string) => {
  const nav = getContext<Nav>(KEY);
  nav?.(page);
};
