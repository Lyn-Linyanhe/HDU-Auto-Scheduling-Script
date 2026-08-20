(function (root, factory) {
  if (typeof module === 'object' && module.exports) {
    module.exports = factory();
  } else {
    root.HDUExponentialBackoff = factory();
  }
})(typeof self !== 'undefined' ? self : this, function () {
  // liveRefreshWaitingSeconds returns the seconds to wait after failureStreak
  // consecutive failed refreshes, starting from baseSeconds and doubling each
  // time up to capSeconds. A user-configured base above the cap is respected
  // (it already exceeds the ceiling).
  function liveRefreshWaitingSeconds(baseSeconds, failureStreak, capSeconds) {
    baseSeconds = Number(baseSeconds);
    if (!(baseSeconds > 0)) baseSeconds = 60;
    failureStreak = Number(failureStreak) || 0;
    if (failureStreak < 0) failureStreak = 0;
    if (failureStreak > 12) failureStreak = 12;
    capSeconds = Number(capSeconds) || 7200;
    var doubled = baseSeconds * Math.pow(2, failureStreak);
    var next = doubled > capSeconds ? capSeconds : doubled;
    return next > baseSeconds ? next : baseSeconds;
  }

  return { liveRefreshWaitingSeconds: liveRefreshWaitingSeconds };
});
