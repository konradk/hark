.pragma library

function unitsForTick(bufferedUnits, intervalMs, targetCatchUpMs, maximumUnits) {
    const buffered = Math.max(0, Math.floor(Number(bufferedUnits)));
    if (buffered === 0)
        return 0;

    const interval = Math.max(1, Number(intervalMs));
    const target = Math.max(interval, Number(targetCatchUpMs));
    const ticks = Math.max(1, Math.ceil(target / interval));
    const maximum = Math.max(1, Math.floor(Number(maximumUnits)));
    return Math.min(buffered, maximum, Math.max(1, Math.ceil(buffered / ticks)));
}
