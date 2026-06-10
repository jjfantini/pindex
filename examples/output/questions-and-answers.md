# Q&A transcript: bitcoin.pdf

Real, unedited output from `pindex ask` against the indexed Bitcoin whitepaper
(`examples/bitcoin.pdf`, 9 pages). The document was indexed first with:

```sh
pindex index bitcoin.pdf --model gpt-4o-mini
```

Answers print to stdout; the `cited pages:` line prints to stderr.

---

## Q1

```sh
pindex ask "How does the network prevent double-spending?" --model gpt-4o-mini
```

**Answer (stdout):**

> The network prevents double-spending through a proof-of-work mechanism and consensus among nodes on the longest chain of transactions.

**Citations (stderr):**

```
cited pages: [2 3 5 12]  (doc: bitcoin.pdf)
```

---

## Q2

```sh
pindex ask "What incentive do nodes have to support the network honestly?" --doc bitcoin.pdf --model gpt-4o-mini
```

**Answer (stdout):**

> Nodes are incentivized to support the network honestly through the creation of new coins and transaction fees.

**Citations (stderr):**

```
cited pages: [4]  (doc: bitcoin.pdf)
```

---

## Q3

```sh
pindex ask "What does the attacker probability analysis in section 11 conclude?" --model gpt-4o-mini --out qa-out
```

**Answer (stdout):**

> The probability of an attacker catching up to the honest chain decreases exponentially with the number of blocks behind (z).

**Citations (stderr):**

```
cited pages: [7 8]  (doc: bitcoin.pdf)
wrote answer to qa-out/bitcoin/answers/What_does_the_attacker_probability_analy_a1f0bfd9.json
```
