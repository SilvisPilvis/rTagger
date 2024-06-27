// #![cfg_attrnot(debug_assertions)]
// #![windows_subsystem = "windows"]
use image::{GenericImageView, Pixel};
use image::io::Reader as ImageReader;
use egui::{Image, ImageData, ColorImage};
use std::fs;
use std::fs::File;
use std::io::Read;
use std::path::Path;

fn load_image_from_path(path: &std::path::Path) -> Result<egui::ColorImage, image::ImageError> {
    let image = image::io::Reader::open(path)?.decode()?;
    let size = [image.width() as _, image.height() as _];
    let image_buffer = image.to_rgba8();
    let pixels = image_buffer.as_flat_samples();
    Ok(egui::ColorImage::from_rgba_unmultiplied(
        size,
        pixels.as_slice(),
    ))
}

fn load_image_from_memory(image_data: &[u8]) -> Result<ColorImage, image::ImageError> {
    let image = image::load_from_memory(image_data)?;
    let size = [image.width() as _, image.height() as _];
    let image_buffer = image.to_rgba8();
    let pixels = image_buffer.as_flat_samples();
    Ok(ColorImage::from_rgba_unmultiplied(
        size,
        pixels.as_slice(),
    ))
}

// fn load_image<P: AsRef<Path>>(path: P) -> Result<image::DynamicImage, image::ImageError> {
//     image::open(path)
// }

fn load_icon(path: &str) -> Vec<u8> {
    let img = image::open(path).unwrap();
    let (width, height) = img.dimensions();
    let mut rgba = Vec::with_capacity((width * height * 4) as usize);
    for (_, _, pixel) in img.pixels() {
        rgba.extend_from_slice(&pixel.to_rgba().0);
    }
    rgba
}

fn main() -> Result<(), eframe::Error> {
    // let icon = load_icon("./icon.png");
    let options = eframe::NativeOptions {
        centered: true,
        // drag_and_drop_support: true,
        // initial_window_size: Some(egui::vec2(400.0, 300.0)),
        // icon_data: Some(eframe::IconData {
        //     rgba: icon.to_vec(),
        //     width: 32,
        //     height: 32,
        // }),
        ..Default::default()
    };
    eframe::run_native(
        "Tagger",
        options,
        Box::new(|_cc| Box::<MyApp>::default()),
    )
}

#[derive(Default)]
struct MyApp {
    // dropped_files: Vec<egui::DroppedFile>,
    // picked_path: Option<String>,
    // key: String,
    // key_file: bool,
    // ratio: u32,
    path: String,
}

fn display_files(ui: &mut egui::Ui, directory_path: &str, ctx: &egui::Context) {
    if let Ok(entries) = fs::read_dir(directory_path) {
        for entry in entries {
            if let Ok(entry) = entry {
                if let Ok(file_type) = entry.file_type() {
                    if file_type.is_file() {
                        let file_name = entry.file_name().into_string().unwrap();

                        ui.with_layout(egui::Layout::top_down(egui::Align::TOP),|ui| {
                            let binding = file_name.to_string();
                            // parts[1] == extension
                            let parts: Vec<&str> = binding.split('.').collect();
                            if (parts[1] == "png") || (parts[1] == "jpg") {
                                // displays image
                                ui.image(format!("file:///{directory_path}/{file_name}"));
                                // displays filename
                                ui.label(&file_name);
                            } else {                
                                // adds icon
                                ui.add(egui::Image::new(egui::include_image!("./bash.png")).rounding(5.0).shrink_to_fit().fit_to_original_size(0.2));
                                // adds filename
                                ui.label(&file_name);
                            }
                        });
                    }
                }
            }
        }
    }
}

struct TileData {
    name: String,
    icon: Vec<u8>,
}

// fn NewTile(ui: &mut egui::Ui, tile_data: &TileData) {
//     ui.image(tile_data.icon.as_ref());
//     ui.label(&tile_data.name);
// }

impl eframe::App for MyApp {

    fn update(&mut self, ctx: &egui::Context, _frame: &mut eframe::Frame) {
        egui::CentralPanel::default().show(ctx, |ui| {

            egui_extras::install_image_loaders(ctx);

            // self.path = r"C:\Users\Silvestrs\Desktop\test".to_string();
            self.path = r"C:\Users\Silvestrs\".to_string();

            // let mut my_tabs = MyTabs::new();
            // let mut tab_viewer = TabViewer { input_text: String::new() };
            // egui_dock::DockArea::new(&mut my_tabs.tree).style(Style::from_egui(ui.style().as_ref())).show_inside(ui, &mut tab_viewer);

            ui.label("Path: ");
            ui.text_edit_singleline( &mut self.path);

            // egui::Ui::with_layout(ui, egui::Layout::right_to_left(egui::Align::TOP), |ui| {
            //     ui.label("Name");
            //     display_files(ui, &self.path, ctx);
            // });

            ui.label("Name");
            // egui::Grid::new("files").show(ui, |ui| {
            //     display_files(ui, &self.path, ctx);
            //     ui.end_row();
            // });

            // ui.with_layout(egui::Layout::left_to_right(egui::Align::LEFT).with_main_wrap(true), |ui| {
            //     display_files(ui, &self.path, ctx);
            // });

            ui.group(|ui| {
                display_files(ui, &self.path, ctx);
            })

            // egui::Ui::horizontal_wrapped(ui,|ui| {
            //     display_files(ui, &self.path, ctx);
            //     ui.label("testogus");
            //     ui.label("testogus");
            //     ui.label("testogus");
            //     ui.label("testogus");
            //     ui.label("testogus");
            //     ui.label("testogus");
            //     ui.label("testogus");
            //     ui.label("testogus");
            //     ui.label("testogus");
            //     ui.label("testogus");
            //     ui.label("testogus");
            //     ui.label("testogus");
            //     ui.label("testogus");
            //     ui.label("testogus");
            //     ui.label("testogus");
            //     ui.label("testogus");
            //     ui.label("testogus");
            //     ui.label("testogus");

            // });

            // let img = image::open("./bash.png").expect("Failed to open image");
            // let size = [img.width() as usize, img.height() as usize];
            // let image_buffer = img.to_rgba8();
            // let pixels = image_buffer.as_flat_samples();
            // let texture = Some(ui.ctx().load_texture(
            //     "icon",
            //     egui::ColorImage::from_rgba_unmultiplied(size, pixels.as_slice()),
            //     egui::TextureOptions::default(),
            //     // egui::TextureOptions::NEAREST
            //     // egui::TextureFilter::Linear,
            // ));
            // let test = "2210b98d4d389170b330b5faf6f8c1d437e0d774.png";
            // ui.image(format!("file:///C:/Users/Silvestrs/Desktop/test/{test}"));
            // ui.image("file:///C:/Users/Silvestrs/Desktop/test/2210b98d4d389170b330b5faf6f8c1d437e0d774.png");
            // ui.image(texture);

            // ui.add(ui.image(egui::include_image!("./bash.png")));

            // ui.add(egui::Image::new(egui::include_image!("./bash.png")).rounding(5.0).fit_to_exact_size(egui::Vec2::new(1.0, 1.0)) );
            
            // egui::Image::new(egui::include_image!("./bash.png"))
            // .rounding(5.0)
            // .tint(egui::Color32::LIGHT_BLUE)
            // .paint_at(ui, egui::Rect::from_center_size(egui::pos2(100.0, 100.0), egui::vec2(100.0, 100.0)));

            

        });
    }
}
