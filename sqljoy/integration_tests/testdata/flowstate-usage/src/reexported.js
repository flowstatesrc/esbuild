import * as nested from "./nested/other";

export const RENAMED = nested.NESTED2;
export {NESTED3 as ALIASED3} from "./nested/other";
export * as nestedAlias from "./nested/other";
export {someFunc} from "./nested/other";