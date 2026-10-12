import re, sys, statistics
d = sys.argv[1]
n = int(sys.argv[2])
rss, wall, txs = [], [], []
for i in range(1, n + 1):
    src = open('%s/actor-%d.txt' % (d, i), encoding='utf-8', errors='replace').read()
    m = re.search(r'Maximum resident set size \(kbytes\): (\d+)', src)
    w = re.search(r'Elapsed.*: (\d+):([\d.]+)', src)
    t = re.search(r'GATE0_TRANSACTIONS=(\d+)', src)
    rss.append(int(m.group(1)) if m else -1)
    if w:
        wall.append(int(w.group(1)) * 60 + float(w.group(2)))
    txs.append(int(t.group(1)) if t else -1)
rss = [r for r in rss if r > 0]
print('n=%d ok=%d/%d tx_modes=%s' % (n, sum(1 for t in txs if t == 3), n, sorted(set(txs))))
print('rss_kb min=%d mean=%d p50=%d max=%d' % (min(rss), sum(rss) // len(rss), sorted(rss)[len(rss) // 2], max(rss)))
print('rss_total_mb=%.0f' % (sum(rss) / 1024))
print('actor_wall_s min=%.1f mean=%.1f max=%.1f' % (min(wall), sum(wall) / len(wall), max(wall)))
