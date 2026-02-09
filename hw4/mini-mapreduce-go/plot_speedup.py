import pandas as pd
import matplotlib.pyplot as plt

df = pd.read_csv("results_concurrent.csv")

# real speedup
df["speedup_real"] = df["map_serial_sum"] / df["map_wall"]

# fix num_chunks to 3 (match your previous plot)
df_fixed = df[df["num_chunks"] == 3]

g = df_fixed.groupby("mapper_count")["speedup_real"].mean().reset_index()

plt.figure()
plt.plot(g["mapper_count"], g["speedup_real"], marker="o")
plt.xlabel("Number of Mappers")
plt.ylabel("Real Speedup (map_serial_sum / map_wall)")
plt.title("Map Stage Real Speedup vs Number of Mappers")
plt.grid(True)
plt.show()
