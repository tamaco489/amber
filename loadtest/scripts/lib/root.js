// GET / を送る (setup で古い版を読むときと、繰り返しの中で数えるときの両方で使う)
import http from "k6/http";

import { baseUrl, timeout } from "./config.js";

export function getRoot() {
  return http.get(`${baseUrl}/`, { timeout: timeout });
}
