import { createRequire } from 'module';
const require = createRequire(import.meta.url);
const gts = require('gts');

export default [
  ...gts
];
