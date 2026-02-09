import json
import re
from collections import Counter

# 1) read book.txt 
with open("book.txt", "r", encoding="utf-8") as f:
    text = f.read().lower()

# 2) baseline word count
words = re.findall(r"[a-z']+", text)
baseline = Counter(words)

# 3) read mapreduce result
with open("result.json") as f:
    mr = json.load(f)

# 4) convert to counter
mr_counter = Counter({k: int(v) for k, v in mr.items()})

# 5) compare
print("baseline unique words:", len(baseline))
print("mapreduce unique words:", len(mr_counter))

diff = baseline - mr_counter
diff_rev = mr_counter - baseline

print("missing words:", len(diff))
print("extra words:", len(diff_rev))

if not diff and not diff_rev:
    print("Word count matches exactly!")
else:
    print("Differences found")
    print("Example missing:", list(diff.items())[:5])
    print("Example extra:", list(diff_rev.items())[:5])
