# -*- coding: utf-8 -*-
import numpy as np
from random import sample
from numpy.random import default_rng
from typing import Container
from itertools import chain, combinations
from tqdm.auto import tqdm


def powerset(iterable):
    s = iterable if isinstance(iterable, Container) else [*iterable]
    return chain.from_iterable(combinations(s, r) for r in range(len(s) + 1))


def main():
    genes = [
        "M55150",
        "U32944",
        "U50136",
        "X95735",
        "M92287",
        "X59350",
        "M28130",
        "M31211",
        "D88422",
        "U46499",
        "X59417",
        "Y00787",
        "M84526",
        "U46751",
        "HG1322",
        "L47738",
        "M80254",
        "D88270",
        "M62762",
        "U05259",
        "M84371",
        "U26266",
        "M22324",
        "M69043",
        "U97105",
        "M63838",
        "M16038",
        "M23197",
        "M89957",
        "M63138",
        "J05243",
        "X70070",
        "X82240",
        "D43948",
        "M83667",
        "X15414",
        "X74570",
        "U40369",
        "D83785",
        "U10323",
    ]

    sample_sizes = range(9, 14, 2)
    for sample_size in tqdm(sample_sizes, total=3, desc="sizes"):
        ps = [*powerset(genes[:sample_size])][1:]
        ps_size = len(ps)
        print(ps_size)
        vals = default_rng().dirichlet(np.ones(ps_size), size=1)[0]
        vals.sort()
        shuffle_ps = []
        prev_len = 1
        local = []
        for p in tqdm(ps, total=ps_size, desc="powerset"):
            lp = len(p)
            local.append(sample(p, lp))
            if lp != prev_len:
                prev_len += 1
                shuffle_ps.extend(local[:])
                local.clear()

        with open(f"data/N{sample_size}", "w+") as out:
            for i in range(ps_size):
                out.write(f"{' '.join(shuffle_ps[i])},{vals[i]}\n")


if __name__ == "__main__":
    main()
