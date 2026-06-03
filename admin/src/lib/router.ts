// 极简 router helper —— 直接调用回调
export type NavFn = (page: string) => void;
export const goto = (setPage: NavFn, page: string) => setPage(page);
