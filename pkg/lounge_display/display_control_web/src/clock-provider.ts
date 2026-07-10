import { Clock, RealTimeClock } from 'standard-ts-lib/src/clock.js';

let appClock: Clock = new RealTimeClock();

export function setAppClock(clock: Clock) {
  appClock = clock;
}

export function getAppClock(): Clock {
  return appClock;
}
