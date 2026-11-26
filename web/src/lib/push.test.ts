import { expect, test } from 'vitest';
import { decodeKey, isIOS } from './push';

test('decodes a base64url VAPID key', () => {
  expect([...decodeKey('BP_-AQ')]).toEqual([4, 255, 254, 1]);
});

test.each([
  ['Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X)', 5, true],
  ['Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)', 5, true],
  ['Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)', 0, false],
  ['Mozilla/5.0 (Linux; Android 15)', 5, false]
])('isIOS %s', (agent, touch, want) => {
  expect(isIOS(agent, touch)).toBe(want);
});
