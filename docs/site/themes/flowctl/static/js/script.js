let currentIndex = 0;

function showSlide(index) {
    const items = document.querySelectorAll('.hero-carousel-item');
    const dots = document.querySelectorAll('.hero-carousel-dots .dot');

    if (!items.length) return;

    if (index >= items.length) {
        currentIndex = 0;
    } else if (index < 0) {
        currentIndex = items.length - 1;
    } else {
        currentIndex = index;
    }

    items.forEach((item, i) => {
        item.classList.remove('active');
        if (i === currentIndex) {
            item.classList.add('active');
        }
    });

    dots.forEach((dot, i) => {
        dot.classList.remove('active');
        if (i === currentIndex) {
            dot.classList.add('active');
        }
    });
}

function moveCarousel(direction) {
    showSlide(currentIndex + direction);
}

function currentSlide(index) {
    showSlide(index);
}

// Modal functions
function openModal(img, caption) {
    const modal = document.getElementById('imageModal');
    const modalImg = document.getElementById('modalImage');
    const modalCaption = document.getElementById('modalCaption');

    modal.style.display = 'block';
    modalImg.src = img.src;
    modalCaption.textContent = caption;

    // Prevent body scroll when modal is open
    document.body.style.overflow = 'hidden';
}

function closeModal() {
    const modal = document.getElementById('imageModal');
    modal.style.display = 'none';

    // Restore body scroll
    document.body.style.overflow = 'auto';
}

// Initialize carousel when DOM is loaded
document.addEventListener('DOMContentLoaded', function() {
    showSlide(0);

    // Add close modal event listeners
    const modal = document.getElementById('imageModal');
    const closeBtn = document.querySelector('.modal-close');

    // Close when clicking the X button
    closeBtn.addEventListener('click', closeModal);

    // Close when clicking outside the image
    modal.addEventListener('click', function(event) {
        if (event.target === modal) {
            closeModal();
        }
    });

    // Close when pressing Escape key
    document.addEventListener('keydown', function(event) {
        if (event.key === 'Escape') {
            closeModal();
        }
    });
});
