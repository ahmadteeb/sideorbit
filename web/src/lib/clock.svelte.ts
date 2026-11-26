// The current time, updated each minute so countdowns stay right.
let now = $state(Date.now());
if (typeof window !== 'undefined') setInterval(() => (now = Date.now()), 60_000);

export const clock = {
  get now() {
    return now;
  }
};
