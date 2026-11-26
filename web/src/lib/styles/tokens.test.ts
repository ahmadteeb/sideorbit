import { readFileSync } from 'node:fs';
import { expect, test } from 'vitest';

const read = (path: string) => readFileSync(new URL(path, import.meta.url), 'utf8');

test('tokens.css matches the design system copy', () => {
  expect(read('./tokens.css')).toBe(read('../../../../design-system/orbit/tokens.css'));
});
