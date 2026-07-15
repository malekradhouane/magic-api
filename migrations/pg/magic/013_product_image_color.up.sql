-- Associate a product image with a specific color (nullable / empty = generic
-- image shown for every color). Enables the storefront gallery to filter photos
-- by the selected color.
ALTER TABLE product_images
    ADD COLUMN color VARCHAR(100) NOT NULL DEFAULT '';

-- Speed up "images for this color" lookups.
CREATE INDEX idx_product_images_color ON product_images (product_id, color);

