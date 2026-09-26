-- Sitasi lengkap per jawaban (termasuk sitasi ke data pasar yang tidak punya chunk_id),
-- agar riwayat chat bisa menampilkan panel sumber tanpa menghitung ulang.
ALTER TABLE chat_messages ADD COLUMN citations JSONB NOT NULL DEFAULT '[]'::jsonb;
