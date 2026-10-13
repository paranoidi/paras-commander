# Every panel listing load is async, including startup

A hanging `ReadDir` on a network mount or autofs must never freeze the UI, so every load goes through the async loader; the first frame draws before any read, and the panel keeps the previous listing until the new one lands. Cost: "pending" state everywhere, and tests must settle listings explicitly.
