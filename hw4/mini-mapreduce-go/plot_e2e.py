import pandas as pd
import matplotlib.pyplot as plt

df = pd.read_csv("results_concurrent.csv")

# already computed in csv, but keep it robust
df["end_to_end_parallel_est"] = df["split"] + df["map_wall"] + df["reduce"]

df_fixed = df[df["num_chunks"] == 3]
g = df_fixed.groupby("mapper_count")["end_to_end_parallel_est"].mean().reset_index()

plt.figure()
plt.plot(g["mapper_count"], g["end_to_end_parallel_est"], marker="o")
plt.xlabel("Number of Mappers")
plt.ylabel("Estimated End-to-End Parallel Time (s)")
plt.title("Estimated End-to-End Parallel Time vs Number of Mappers")
plt.grid(True)
plt.show()
