importScripts('shared.js');

const hdu = globalThis.HDU;

function signatureForItems(items) {
  return items.map((item) => item.id).sort().join('|');
}

self.onmessage = (event) => {
  const { id, type, groups, state, limit, context } = event.data || {};
  try {
    if (type === 'estimate') {
      self.postMessage({
        id,
        ok: true,
        result: hdu.estimateSolutions(groups || [], state || {}, limit),
      });
      return;
    }

    if (type === 'generate') {
      const generated = hdu.generateSolutions(groups || [], state || {}, limit);
      self.postMessage({
        id,
        ok: true,
        result: {
          ...generated,
          results: generated.results.map((solution) => ({
            ...solution,
            signature: signatureForItems(solution.items),
          })),
        },
      });
      return;
    }

    if (type === 'diagnose') {
      self.postMessage({
        id,
        ok: true,
        result: hdu.diagnoseNoSolutions(groups || [], state || {}, context || {}),
      });
      return;
    }

    throw new Error(`Unknown worker job: ${type}`);
  } catch (error) {
    self.postMessage({
      id,
      ok: false,
      error: String(error?.message || error),
    });
  }
};
